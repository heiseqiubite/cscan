package gogo

import (
	"context"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/model"
	sdkcyberhub "github.com/chainreactors/sdk/pkg/cyberhub"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"sigs.k8s.io/yaml"
)

type GogoSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGogoSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GogoSyncLogic {
	return &GogoSyncLogic{ctx: ctx, svcCtx: svcCtx}
}

type SyncResult struct {
	Added   int
	Updated int
	Skipped int
}

func (l *GogoSyncLogic) SyncFingers() (*SyncResult, error) {
	cfg := l.svcCtx.Config.Cyberhub
	logx.Infof("[GogoSync] Starting sync, URL=%s, Key=%s", cfg.URL, cfg.Key)
	client := sdkcyberhub.NewClient(cfg.URL, cfg.Key, 300*time.Second)

	fingersData, _, err := client.ExportFingers(l.ctx, "")
	if err != nil {
		logx.Errorf("[GogoSync] ExportFingers failed: %v", err)
		return nil, err
	}
	logx.Infof("[GogoSync] ExportFingers returned %d fingers", len(fingersData))

		result := &SyncResult{}
	for i, finger := range fingersData {
			logx.Infof("[GogoSync] Processing finger %d: name=%s", i, finger.Name)
		name := finger.Name
		source := "cyberhub"
		data, err := yaml.Marshal(finger)
		if err != nil {
			logx.Errorf("[GogoSync] Marshal finger %s to yaml failed: %v", name, err)
			continue
		}

		doc := &model.GogoFinger{
			Name:    name,
			Source:  source,
			Data:    data,
			Enabled: true,
		}

		// Check if already exists
		filter := bson.M{"name": name, "source": source}
		existing, _ := l.svcCtx.GogoFingerModel.Find(l.ctx, filter, 0, 0)
		if len(existing) > 0 {
			// Check if update needed
			if string(existing[0].Data) != string(data) {
				doc.ID = existing[0].ID
				doc.CreateTime = existing[0].CreateTime
				result.Updated++
			} else {
				result.Skipped++
				continue
			}
		} else {
			result.Added++
		}

		if err := l.svcCtx.GogoFingerModel.Upsert(l.ctx, doc); err != nil {
			logx.Errorf("[GogoSync] Upsert finger %s failed: %v", name, err)
		}
	}

	return result, nil
}

func (l *GogoSyncLogic) SyncPocs() (*SyncResult, error) {
	cfg := l.svcCtx.Config.Cyberhub
	client := sdkcyberhub.NewClient(cfg.URL, cfg.Key, 300*time.Second)

	pocResponses, err := client.ExportPOCs(l.ctx, nil, nil, "", "")
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}
	for _, resp := range pocResponses {
		tpl := resp.GetTemplate()
		if tpl == nil {
			continue
		}

		name := tpl.Info.Name
		source := "cyberhub"
		data, err := yaml.Marshal(tpl)
		if err != nil {
			logx.Errorf("[GogoSync] Marshal POC %s to yaml failed: %v", name, err)
			continue
		}

		doc := &model.GogoPoc{
			Name:     name,
			Source:   source,
			Data:     data,
			Enabled:  true,
			Severity: getSeverity(tpl.Info.Severity),
			Tags:     strings.Split(tpl.Info.Tags, ","),
		}

		// Check if already exists
		filter := bson.M{"name": name, "source": source}
		existing, _ := l.svcCtx.GogoPocModel.Find(l.ctx, filter, 0, 0)
		if len(existing) > 0 {
			if string(existing[0].Data) != string(data) {
				doc.ID = existing[0].ID
				doc.CreateTime = existing[0].CreateTime
				result.Updated++
			} else {
				result.Skipped++
				continue
			}
		} else {
			result.Added++
		}

		if err := l.svcCtx.GogoPocModel.Upsert(l.ctx, doc); err != nil {
			logx.Errorf("Upsert gogo poc failed: %v", err)
		}
	}

	return result, nil
}

func getSeverity(severity string) string {
	if severity == "" {
		return "info"
	}
	return severity
}