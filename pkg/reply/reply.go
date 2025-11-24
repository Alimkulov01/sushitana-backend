package reply

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"sushitana/pkg/logger"
)

var Module = fx.Invoke(New)

var iLogger logger.Logger

type Params struct {
	fx.In
	Logger logger.Logger
}

func New(params Params) {
	iLogger = params.Logger
}

func Json(w http.ResponseWriter, status int, data interface{}) {

	reply, err := json.Marshal(data)
	if err != nil {
		iLogger.Error(context.TODO(), "err on json.Marshal", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(reply)
}
