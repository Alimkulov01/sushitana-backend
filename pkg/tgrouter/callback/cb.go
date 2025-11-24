package callback

import (
	"fmt"
	"log/slog"
)

type CallbackData struct {
	Query string `json:"query"`
	Value string `json:"value"`
}

func (cd CallbackData) String() string {
	return fmt.Sprintf(`query:%s , value:%s`, cd.Query, cd.Value)
}

func Query(data string) string {
	var cd CallbackData
	_, err := fmt.Sscanf(data, `query:%s , value:%s`, &cd.Query, &cd.Value)
	if err != nil {
		slog.Error("failed to parse callback data", "data", data, "err", err)
		return ""
	}

	return cd.Query
}

func Value(data string) string {
	var cd CallbackData
	_, err := fmt.Sscanf(data, `query:%s , value:%s`, &cd.Query, &cd.Value)
	if err != nil {
		slog.Error("failed to parse callback data", "data", data, "err", err)
		return ""
	}

	return cd.Value
}
