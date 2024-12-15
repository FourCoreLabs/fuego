package fuego

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

type GroupOption struct {
	HideTag bool
	Tag     openapi3.Tag
}

func WithoutTag() func(opt *GroupOption) {
	return func(opt *GroupOption) {
		opt.HideTag = true
	}
}

func WithTag(tag openapi3.Tag) func(opt *GroupOption) {
	return func(opt *GroupOption) {
		opt.Tag = tag
	}
}

func WithName(name string) func(opt *GroupOption) {
	return func(opt *GroupOption) {
		opt.Tag.Name = name
	}
}

func WithDescription(desc string) func(opt *GroupOption) {
	return func(opt *GroupOption) {
		opt.Tag.Description = desc
	}
}

// Group allows grouping routes under a common path.
// Middlewares are scoped to the group.
// For example:
//
//	s := fuego.NewServer()
//	viewsRoutes := fuego.Group(s, "")
//	apiRoutes := fuego.Group(s, "/api")
//	// Registering a middlewares scoped to /api only
//	fuego.Use(apiRoutes, myMiddleware)
//	// Registering a route under /api/users
//	fuego.Get(apiRoutes, "/users", func(c fuego.ContextNoBody) (ans, error) {
//		return ans{Ans: "users"}, nil
//	})
//	s.Run()
func (group *RouterGroup) Group(path string, opts ...func(*GroupOption)) *RouterGroup {
	grpOpt := GroupOption{}
	for _, opt := range opts {
		opt(&grpOpt)
	}

	newGroup := group.newRouteGroup(path, grpOpt)

	if newGroup.groupTag != "" && !group.server.disableAutoGroupTags && !group.DisableOpenapi {
		if !grpOpt.HideTag {
			group.server.OpenApiSpec.Tags = append(group.server.OpenApiSpec.Tags, &grpOpt.Tag)
		}
	}

	return newGroup
}

func Group(group *RouterGroup, path string, opts ...func(*GroupOption)) *RouterGroup {
	return group.Group(path, opts...)
}

type Route[ResponseBody any, RequestBody any] struct {
	Operation  *openapi3.Operation // GENERATED OpenAPI operation, do not set manually in Register function. You can change it after the route is registered.
	All        bool                // define route to all HTTP methods. If true, ignore Method
	Method     string              // HTTP method (GET, POST, PUT, PATCH, DELETE)
	Path       string              // URL path. Will be prefixed by the base path of the server and the group path if any
	mainRouter *Server             // ref to the main router, used to register the route in the OpenAPI spec
}

// Capture all methods (GET, POST, PUT, PATCH, DELETE) and register a controller.
func All[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route[T, B] {
	return Register(s, Route[T, B]{
		Path: path,
		All:  true,
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Get[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route[T, B] {
	return Register(s, Route[T, B]{
		Method: http.MethodGet,
		Path:   path,
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Post[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route[T, B] {
	return Register(s, Route[T, B]{
		Method: http.MethodPost,
		Path:   path,
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Delete[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route[T, B] {
	return Register(s, Route[T, B]{
		Method: http.MethodDelete,
		Path:   path,
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Put[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route[T, B] {
	return Register(s, Route[T, B]{
		Method: http.MethodPut,
		Path:   path,
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Patch[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route[T, B] {
	return Register(s, Route[T, B]{
		Method: http.MethodPatch,
		Path:   path,
	}, FuegoHandler(s.server, controller), middlewares...)
}

func AllGin[Body, Return any](s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route[Return, Body] {
	return Register(s, Route[Return, Body]{
		All:  true,
		Path: path,
	}, controller, middlewares...)
}

func GetGin[Body, Return any](s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route[Return, Body] {
	return Register(s, Route[Return, Body]{
		Method: http.MethodGet,
		Path:   path,
	}, controller, middlewares...)
}

func PostGin[Body, Return any](s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route[Return, Body] {
	return Register(s, Route[Return, Body]{
		Method: http.MethodPost,
		Path:   path,
	}, controller, middlewares...)
}

func PutGin[Body, Return any](s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route[Return, Body] {
	return Register(s, Route[Return, Body]{
		Method: http.MethodPut,
		Path:   path,
	}, controller, middlewares...)
}

func DeleteGin[Body, Return any](s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route[Return, Body] {
	return Register(s, Route[Return, Body]{
		Method: http.MethodPatch,
		Path:   path,
	}, controller, middlewares...)
}

func PatchGin[Body, Return any](s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route[Return, Body] {
	return Register(s, Route[Return, Body]{
		Method: http.MethodPatch,
		Path:   path,
	}, controller, middlewares...)
}

func Register[Body, Return any](group *RouterGroup, route Route[Return, Body], controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route[Return, Body] {
	route.mainRouter = group.server
	handlers := append([]gin.HandlerFunc{controller}, middlewares...)

	if route.All || route.Method == "" {
		group.rg.Any(route.Path, handlers...)
	} else {
		group.rg.Handle(route.Method, route.Path, handlers...)
	}

	if group.DisableOpenapi || route.Method == "" || route.All {
		return route
	}

	route.Path = ginToStdPath(group.rg.BasePath() + route.Path)

	var err error
	route.Operation, err = RegisterOpenAPIOperation(group, route)
	if err != nil {
		slog.Warn("error documenting openapi operation", "error", err)
	}

	if route.Operation.OperationID == "" {
		route.Operation.OperationID = route.Method + "_" + strings.ReplaceAll(strings.ReplaceAll(route.Path, "{", ":"), "}", "")
	}

	route.addPermissionDesc()
	return route
}

func Use(s *RouterGroup, middlewares ...gin.HandlerFunc) {
	s.Use(middlewares...)
}

func (group *RouterGroup) Use(middlewares ...gin.HandlerFunc) {
	group.rg.Use(middlewares...)
}

func ginToStdPath(path string) string {
	builder := strings.Builder{}
	builder.Grow(len(path))

	for len(path) > 0 {
		colIdx := strings.IndexRune(path, ':')
		if colIdx < 0 {
			builder.WriteString(path)
			break
		}

		builder.WriteString(path[:colIdx])
		path = path[colIdx+1:]
		end := strings.IndexRune(path, '/')
		if end < 0 {
			end = len(path)
		}

		builder.WriteRune('{')
		builder.WriteString(path[:end])
		builder.WriteRune('}')

		path = path[end:]
	}

	return builder.String()
}

func stdToGinPath(path string) string {
	replacer := strings.NewReplacer("{", ":", "}", "")
	return replacer.Replace(path)
}
