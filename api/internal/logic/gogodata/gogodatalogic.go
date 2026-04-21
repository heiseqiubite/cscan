package gogodata

import (
	"bytes"
	"context"

	"cscan/api/internal/svc"
)

type GogoDataLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGogoDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GogoDataLogic {
	return &GogoDataLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetFingersData 获取启用的 finger 数据（合并后的 yaml bytes，每条用 --- 分隔）
func (l *GogoDataLogic) GetFingersData() ([]byte, error) {
	var buf bytes.Buffer
	err := l.svcCtx.GogoFingerModel.GetAllData(l.ctx, func(data []byte) error {
		buf.Write([]byte("---\n"))
		buf.Write(data)
		buf.Write([]byte("\n"))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GetPocsData 获取启用的 poc 数据（合并后的 yaml bytes，每条用 --- 分隔）
func (l *GogoDataLogic) GetPocsData() ([]byte, error) {
	var buf bytes.Buffer
	err := l.svcCtx.GogoPocModel.GetAllData(l.ctx, func(data []byte) error {
		buf.Write([]byte("---\n"))
		buf.Write(data)
		buf.Write([]byte("\n"))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
