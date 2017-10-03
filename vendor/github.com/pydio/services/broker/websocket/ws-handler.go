package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/micro/protobuf/jsonpb"
	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/views"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"gopkg.in/olahol/melody.v1"
)

type WebsocketHandler struct {
	Port           int
	Server         *gin.Engine
	Websocket      *melody.Melody
	RootNodesCache map[string]*tree.Node
	TreeClient     tree.NodeProviderClient
	Router         *views.Router
}

func NewWebSocketHandler(port int) *WebsocketHandler {
	w := &WebsocketHandler{
		Port: port,
	}
	w.InitHandlers()
	return w
}

func (w *WebsocketHandler) InitHandlers() {

	w.Server = gin.Default()
	w.Websocket = melody.New()
	w.Websocket.Config.MaxMessageSize = 2048

	w.Server.GET("/ws", func(c *gin.Context) {
		w.Websocket.HandleRequest(c.Writer, c.Request)
	})

	w.Websocket.HandleError(func(session *melody.Session, i error) {
		log.Logger(context.Background()).Error("HandleError", zap.Error(i))
		ClearSession(session)
	})

	w.Websocket.HandleClose(func(session *melody.Session, i int, i2 string) error {
		ClearSession(session)
		return nil
	})

	w.Websocket.HandleMessage(func(session *melody.Session, bytes []byte) {

		msg := &Message{}
		e := json.Unmarshal(bytes, msg)
		log.Logger(context.Background()).Info("After Unmarshall", zap.Any("msg", msg), zap.Error(e))
		if e != nil {
			session.CloseWithMsg(NewErrorMessage(e))
			return
		}
		switch msg.Type {
		case MsgSubscribe:

			if msg.JWT == "" {
				session.CloseWithMsg(NewErrorMessageString("Empty JWT"))
				log.Logger(context.Background()).Error("Empty JWT")
				return
			}
			ctx := context.Background()
			verifier := auth.DefaultJWTVerifier()
			ctx, claims, e := verifier.Verify(ctx, msg.JWT)
			if e != nil {
				log.Logger(context.Background()).Error("Invalid JWT")
				session.CloseWithMsg(NewErrorMessage(e))
				return
			}
			UpdateSessionFromClaims(session, claims)

		case MsgUnsubscribe:

			ClearSession(session)

		default:
			return
		}

	})

	w.Server.GET("/", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "broker/websocket/test.html")
	})

}

func (w *WebsocketHandler) Run() {
	w.Server.Run(fmt.Sprintf(":%d", w.Port))
}

func (w *WebsocketHandler) BroadcastEvent(ctx context.Context, event *tree.NodeChangeEvent) {

	// Here events come with real full path

	w.Websocket.BroadcastFilter([]byte(`"dump"`), func(session *melody.Session) bool {

		value, ok := session.Get(SessionWorkspacesKey)
		if !ok || value == nil {
			return false
		}
		workspaces := value.(map[string]*idm.Workspace)

		n1, t1 := w.SessionCanSeeNode(ctx, workspaces, event.Target, true)
		n2, t2 := w.SessionCanSeeNode(ctx, workspaces, event.Source, false)
		// Depending on node, broadcast now
		if t1 || t2 {
			log.Logger(ctx).Debug("Root is under authorized path, broadcasting event to this session", zap.Any("target", n1), zap.Any("source", n2))

			// We have to filter the event for this context
			marshaler := &jsonpb.Marshaler{}
			s, _ := marshaler.MarshalToString(&tree.NodeChangeEvent{
				Type:   event.Type,
				Target: n1,
				Source: n2,
			})
			data := []byte(s)

			session.Write(data)
			return true
		}

		return false
	})

}

func (w *WebsocketHandler) SessionCanSeeNode(ctx context.Context, workspaces map[string]*idm.Workspace, node *tree.Node, refresh bool) (*tree.Node, bool) {
	if node == nil {
		return node, false
	}
	if node.GetStringMeta("name") == ".__pydio" {
		return node, false
	}
	for _, workspace := range workspaces {
		roots := workspace.RootNodes
		for _, root := range roots {
			if parent, ok := w.NodeIsChildOfRoot(ctx, node, root); ok {

				log.Logger(ctx).Debug("Before Filter", zap.Any("node", node))
				var newNode *tree.Node
				if refresh {
					respNode, err := w.TreeClient.ReadNode(ctx, &tree.ReadNodeRequest{Node: node})
					if err != nil {
						return nil, false
					}
					newNode = respNode.Node
				} else {
					newNode = &tree.Node{Uuid: node.Uuid, Path: node.Path}
				}
				w.Router.WrapCallback(func(inputFilter views.NodeFilter, outputFilter views.NodeFilter) error {
					branchInfo := views.BranchInfo{}
					branchInfo.Workspace = *workspace
					branchInfo.Root = parent
					ctx = views.WithBranchInfo(ctx, "in", branchInfo)
					outputFilter(ctx, newNode, "in")
					return nil
				})
				log.Logger(ctx).Debug("After Filter", zap.Any("node", newNode))
				return newNode, true

			}
		}
	}
	return nil, false
}

func (w *WebsocketHandler) NodeIsChildOfRoot(ctx context.Context, node *tree.Node, rootId string) (*tree.Node, bool) {

	if root := w.getRoot(ctx, rootId); root != nil {
		return root, strings.HasPrefix(node.Path, root.Path)
	}
	return nil, false

}

func (w *WebsocketHandler) getRoot(ctx context.Context, rootId string) *tree.Node {

	if w.RootNodesCache == nil {
		w.RootNodesCache = make(map[string]*tree.Node)
	}
	if node, ok := w.RootNodesCache[rootId]; ok {
		return node
	}
	resp, e := w.TreeClient.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{Uuid: rootId}})
	if e == nil && resp.Node != nil {
		return resp.Node
	}
	return nil

}
