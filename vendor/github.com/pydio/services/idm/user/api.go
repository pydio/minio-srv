package user

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/micro/go-micro"
	"github.com/micro/micro/api/proto"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/service"
	serviceproto "github.com/pydio/services/common/service/proto"

	"github.com/micro/cli"
	"golang.org/x/net/context"

	"github.com/micro/go-micro/errors"
)

type User struct {
	client idm.UserServiceClient
}

func userBuilder(service micro.Service) interface{} {
	return &User{
		client: idm.NewUserServiceClient(common.SERVICE_USER, service.Client()),
	}
}

func (s *User) Bind(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	data := req.GetPost()

	var login, password string

	if arg, ok := data["username"]; ok {
		login = arg.Values[0]
	}

	if arg, ok := data["password"]; ok {
		password = arg.Values[0]
	}

	if len(login) == 0 || len(password) == 0 {
		return errors.BadRequest(common.SERVICE_USER, "Missing arguments for bind request")
	}

	log.Logger(ctx).Info("Bind", zap.String("login", login), zap.String("password", password))

	resp, er := s.client.BindUser(ctx, &idm.BindUserRequest{UserName: login, Password: password})

	if er != nil {
		log.Logger(ctx).Error("Bind", zap.Error(er))

		parsed := errors.Parse(er.Error())
		if parsed.Code > 0 {
			rsp.StatusCode = parsed.Code
			rsp.Body = parsed.Status + ": " + parsed.Detail
		} else {
			rsp.StatusCode = 500
			rsp.Body = "Internal Server Error"
		}
	} else {
		log.Logger(ctx).Debug("Bind", zap.Any("resp", resp))

		rsp.StatusCode = 200
		rsp.Header = make(map[string]*go_micro_api.Pair, 1)
		rsp.Header["Content-type"] = &go_micro_api.Pair{
			Key:    "Content-type",
			Values: []string{"application/json; charset=utf8"},
		}
		b, _ := json.Marshal(resp.User)
		rsp.Body = string(b)
	}
	return nil
}

func (s *User) Put(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	user := new(idm.User)
	err := json.Unmarshal([]byte(req.Body), &user)
	if err != nil {
		return err
	}

	_, er := s.client.CreateUser(ctx, &idm.CreateUserRequest{
		User: user,
	})

	if er != nil {
		return nil
	}

	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

func (s *User) Delete(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	data := req.GetPost()

	var login, password, groupPath string

	if arg, ok := data["login"]; ok {
		login = arg.Values[0]
	}

	if arg, ok := data["password"]; ok {
		password = arg.Values[0]
	}

	if arg, ok := data["group_path"]; ok {
		groupPath = arg.Values[0]
	}

	if login == "" || password == "" {
		return fmt.Errorf("Wrong arguments")
	}

	query, _ := ptypes.MarshalAny(&idm.UserSingleQuery{
		Login:     login,
		Password:  password,
		GroupPath: groupPath,
	})

	if _, err := s.client.DeleteUser(ctx, &idm.DeleteUserRequest{
		Query: &serviceproto.Query{
			SubQueries: []*any.Any{query},
		},
	}); err != nil {
		return fmt.Errorf("Could not delete user")
	}
	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

func (s *User) Search(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	data := req.GetGet()

	var login, password, groupPath string

	if arg, ok := data["login"]; ok {
		login = arg.Values[0]
	}

	if arg, ok := data["password"]; ok {
		password = arg.Values[0]
	}

	if arg, ok := data["group_path"]; ok {
		groupPath = arg.Values[0]
	}

	if login == "" {
		return fmt.Errorf("Wrong arguments")
	}

	query, _ := ptypes.MarshalAny(&idm.UserSingleQuery{
		Login:     login,
		Password:  password,
		GroupPath: groupPath,
	})

	stream, err := s.client.SearchUser(ctx, &idm.SearchUserRequest{
		Query: &serviceproto.Query{
			SubQueries: []*any.Any{query},
		},
	})

	if err != nil {
		return fmt.Errorf("Could not search acls")
	}

	defer stream.Close()

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*go_micro_api.Pair, 1)
	rsp.Header["Content-type"] = &go_micro_api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	var users []*idm.User
	for {
		response, err := stream.Recv()

		if err != nil {
			break
		}

		users = append(users, response.GetUser())
	}

	b, _ := json.Marshal(users)
	rsp.Body = string(b)

	return nil
}

// Starts the API
// Then Start :
// micro --client=grpc api --namespace="pydio.service.api"
//
// Then call e.g. http://localhost:8080/meta/read?uuid="existing-uuid" or ?path="datasource/path/to/node"
func NewAPIService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(userBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"user"))

	return srv, nil

}
