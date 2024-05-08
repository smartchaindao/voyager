package api

import (
	"fmt"
	"net/http"

	"github.com/yanhuangpai/voyager/pkg/jsonhttp"
)

type isLatestClientResponse struct {
	IsLatest uint `json:"IsLatest"`
}

func (s *server) isLatestClientVersion(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Step into isLatestClientVersion")
	var isLatest uint
	if s.flg.VersionCheck {
		// VersionCheck is ture
		isLatest = 1
	} else {
		// VersionCheck is false
		isLatest = 0
	}

	jsonhttp.OK(w, isLatestClientResponse{
		IsLatest: isLatest,
	})
}
