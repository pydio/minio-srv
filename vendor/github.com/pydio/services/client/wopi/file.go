package wopi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/views"
	"go.uber.org/zap"
)

type File struct {
	BaseFileName    string
	OwnerId         string
	Size            int64
	UserId          string
	Version         string
	UserFriendlName string
	UserCanWrite    bool
	PydioPath       string
}

func findNodeFromRequest(r *http.Request) (*tree.Node, error) {

	vars := mux.Vars(r)
	uuid := vars["uuid"]
	if uuid == "" {
		return nil, errors.InternalServerError(common.SERVICE_API_NAMESPACE_+".wopi", "Cannot find uuid in parameters")
	}

	// Now go through all the authorization mechanisms
	resp1, err1 := viewsRouter.ReadNode(r.Context(), &tree.ReadNodeRequest{
		Node: &tree.Node{Uuid: uuid},
	})
	if err1 != nil {
		return nil, err1
	}
	log.Logger(r.Context()).Debug("Router Response node", zap.Any("node", resp1.Node))

	return resp1.Node, nil

}

func Download(w http.ResponseWriter, r *http.Request) {

	n, err := findNodeFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	read, er := viewsRouter.GetObject(r.Context(), n, &views.GetRequestData{StartOffset: 0, Length: -1})
	if er != nil {
		log.Logger(r.Context()).Error("Error while getting object: " + er.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", n.GetSize()))
	defer read.Close()
	written, err := io.Copy(w, read)
	log.Logger(r.Context()).Debug("Sent data to output", zap.Int64("Data Length", written), zap.Error(err))

}

func GetNodeInfos(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	n, err := findNodeFromRequest(r)
	if err != nil {
		log.Logger(r.Context()).Error(err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Build File from Node
	f := &File{
		BaseFileName:    n.GetStringMeta("name"),
		OwnerId:         "ownerIdName",
		Size:            n.GetSize(),
		UserId:          "userId",
		UserCanWrite:    true,
		UserFriendlName: "Nice Name For John",
		Version:         fmt.Sprintf("%d", n.GetModTime().Unix()),
		PydioPath:       n.Path,
	}

	data, _ := json.Marshal(f)
	w.Write(data)

}

func UploadStream(w http.ResponseWriter, r *http.Request) {

	log.Logger(r.Context()).Debug("Upload Stream")

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	n, err := findNodeFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var size int64
	if h, ok := r.Header["Content-Length"]; ok && len(h) > 0 {
		size, _ = strconv.ParseInt(h[0], 10, 64)
	}

	written, err := viewsRouter.PutObject(r.Context(), n, r.Body, &views.PutRequestData{
		Size: size,
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Logger(r.Context()).Debug("Read data from input", zap.Int64("Data Length", written), zap.Error(err))

	w.WriteHeader(http.StatusOK)

}
