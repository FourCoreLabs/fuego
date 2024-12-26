package fuego

import (
	"net/http"

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

type Route struct {
	Operation  *openapi3.Operation // GENERATED OpenAPI operation, do not set manually in Register function. You can change it after the route is registered.
	All        bool                // define route to all HTTP methods. If true, ignore Method
	Method     string              // HTTP method (GET, POST, PUT, PATCH, DELETE)
	Path       string              // URL path. Will be prefixed by the base path of the server and the group path if any
	mainRouter *Server             // ref to the main router, used to register the route in the OpenAPI spec
	Group      *RouterGroup
	Response   any
	Request    any
}

// Capture all methods (GET, POST, PUT, PATCH, DELETE) and register a controller.
func All[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Path:     path,
		All:      true,
		Request:  new(B),
		Response: new(T),
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Get[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method:   http.MethodGet,
		Path:     path,
		Request:  new(B),
		Response: new(T),
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Post[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method:   http.MethodPost,
		Path:     path,
		Request:  new(B),
		Response: new(T),
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Delete[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method:   http.MethodDelete,
		Path:     path,
		Request:  new(B),
		Response: new(T),
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Put[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method:   http.MethodPut,
		Path:     path,
		Request:  new(B),
		Response: new(T),
	}, FuegoHandler(s.server, controller), middlewares...)
}

func Patch[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method:   http.MethodPatch,
		Path:     path,
		Request:  new(B),
		Response: new(T),
	}, FuegoHandler(s.server, controller), middlewares...)
}

func AllGin(s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		All:  true,
		Path: path,
	}, controller, middlewares...)
}

func GetGin(s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method: http.MethodGet,
		Path:   path,
	}, controller, middlewares...)
}

func PostGin(s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method: http.MethodPost,
		Path:   path,
	}, controller, middlewares...)
}

func PutGin(s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method: http.MethodPut,
		Path:   path,
	}, controller, middlewares...)
}

func DeleteGin(s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method: http.MethodPatch,
		Path:   path,
	}, controller, middlewares...)
}

func PatchGin(s *RouterGroup, path string, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route {
	return Register(s, Route{
		Method: http.MethodPatch,
		Path:   path,
	}, controller, middlewares...)
}

func Register(group *RouterGroup, route Route, controller gin.HandlerFunc, middlewares ...gin.HandlerFunc) Route {
	route.mainRouter = group.server
	route.Group = group
	route.Operation = openapi3.NewOperation()

	handlers := append([]gin.HandlerFunc{controller}, middlewares...)

	if route.All || route.Method == "" {
		group.rg.Any(route.Path, handlers...)
	} else {
		group.rg.Handle(route.Method, route.Path, handlers...)
	}

	// route.Path = ginToStdPath(group.rg.BasePath() + route.Path)
	return route
}

func Use(s *RouterGroup, middlewares ...gin.HandlerFunc) {
	s.Use(middlewares...)
}

func (group *RouterGroup) Use(middlewares ...gin.HandlerFunc) {
	group.rg.Use(middlewares...)
}
