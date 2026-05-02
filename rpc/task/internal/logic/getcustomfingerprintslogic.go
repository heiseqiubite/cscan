package logic

import (
	"context"

	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type GetCustomFingerprintsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCustomFingerprintsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCustomFingerprintsLogic {
	return &GetCustomFingerprintsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取自定义指纹
func (l *GetCustomFingerprintsLogic) GetCustomFingerprints(in *pb.GetCustomFingerprintsReq) (*pb.GetCustomFingerprintsResp, error) {
	fingerprints := make([]*pb.FingerprintDocument, 0)

	// 构建查询条件
	filter := bson.M{}
	if in.EnabledOnly {
		filter["enabled"] = true
	}

	// 获取指纹列表
	fps, err := l.svcCtx.FingerprintModel.Find(l.ctx, filter, 0, 0)
	if err != nil {
		l.Logger.Errorf("GetCustomFingerprints: failed to get fingerprints: %v", err)
		return &pb.GetCustomFingerprintsResp{
			Success: false,
			Message: "获取指纹失败: " + err.Error(),
		}, nil
	}

	// 转换为protobuf格式（使用指针避免拷贝 protobuf 内部锁）
	for _, fp := range fps {
		pbFp := &pb.FingerprintDocument{
			Id:        fp.Id.Hex(),
			Name:      fp.Name,
			Category:  fp.Category,
			Rule:      fp.Rule,
			Source:    fp.Source,
			Headers:   fp.Headers,
			Cookies:   fp.Cookies,
			Html:      fp.HTML,
			Scripts:   fp.Scripts,
			ScriptSrc: fp.ScriptSrc,
			Meta:      fp.Meta,
			Css:       fp.CSS,
			Url:       fp.URL,
			IsBuiltin: fp.IsBuiltin,
			Enabled:   fp.Enabled,
		}
		fingerprints = append(fingerprints, pbFp)
	}

	return &pb.GetCustomFingerprintsResp{
		Success:      true,
		Message:      "success",
		Fingerprints: fingerprints,
		Count:        int32(len(fingerprints)),
	}, nil
}
