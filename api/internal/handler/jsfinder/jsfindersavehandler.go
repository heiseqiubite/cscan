package jsfinder

import (
	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"
	"encoding/json"
	"net/http"
)

// SaveJSFinderResultHandler 处理保存 JSFinder 扫描结果请求
func SaveJSFinderResultHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SaveJSFinderResultReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		err := l.SaveJSFinderResult(&req)
		if err != nil {
			response.Error(w, err)
			return
		}

		response.Success(w, map[string]interface{}{})
	}
}
