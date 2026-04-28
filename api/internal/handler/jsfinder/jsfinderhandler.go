package jsfinder

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// JSFinderConfigGetHandler 获取 JSFinder 配置
func JSFinderConfigGetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewJSFinderConfigLogic(r.Context(), svcCtx)
		resp, err := l.Get()
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// JSFinderConfigSaveHandler 保存 JSFinder 配置
func JSFinderConfigSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JSFinderConfigSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewJSFinderConfigLogic(r.Context(), svcCtx)
		resp, err := l.Save(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// JSFinderConfigResetHandler 重置为内置默认值
func JSFinderConfigResetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewJSFinderConfigLogic(r.Context(), svcCtx)
		resp, err := l.Reset()
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
