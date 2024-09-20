package fuego

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
)

type contextKeyType string

const GinContextKey contextKeyType = gin.ContextKey

// Run starts the server.
// It is blocking.
// It returns an error if the server could not start (it could not bind to the port for example).
// It also generates the OpenAPI spec and outputs it to a file, the UI, and a handler (if enabled).
func (s *Server) Run(addr string) error {
	// s.setup()
	return s.engine.Run(addr)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}

// initializes any Context type with the base ContextNoBody context.
//
//	var ctx ContextWithBody[any] // does not work because it will create a ContextWithBody[any] with a nil value
func initContext[Contextable ctx[Body], Body any](baseContext ContextNoBody) Contextable {
	var c Contextable

	switch any(c).(type) {
	case ContextNoBody:
		return any(baseContext).(Contextable)
	case *ContextNoBody:
		return any(&baseContext).(Contextable)
	case *ContextWithBody[Body]:
		return any(&ContextWithBody[Body]{
			ContextNoBody: baseContext,
		}).(Contextable)
	default:
		panic("unknown type")
	}
}

func wrapGinContext(handler http.Handler) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Request.Pattern = ctx.FullPath()
		ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), GinContextKey, ctx))
		handler.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

// HTTPHandler converts a Fuego controller into a http.HandlerFunc.
func HTTPHandler[ReturnType, Body any, Contextable ctx[Body]](s *Server, controller func(c Contextable) (ReturnType, error)) http.HandlerFunc {
	// Just a check, not used at request time
	baseContext := *new(Contextable)
	if reflect.TypeOf(baseContext) == nil {
		slog.Info(fmt.Sprintf("context is nil: %v %T", baseContext, baseContext))
		panic("ctx must be provided as concrete type (not interface). ContextNoBody, ContextWithBody[any], ContextFull[any, any], ContextWithQueryParams[any] are supported")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// CONTEXT INITIALIZATION
		var templates *template.Template
		if s.template != nil {
			templates = template.Must(s.template.Clone())
		}

		ginCtx, _ := r.Context().Value(GinContextKey).(*gin.Context)

		ctx := initContext[Contextable](ContextNoBody{
			Req: r,
			Res: w,
			readOptions: readOptions{
				DisallowUnknownFields: s.DisallowUnknownFields,
				MaxBodySize:           s.maxBodySize,
			},
			fs:        s.fs,
			templates: templates,
			ginCtx:    ginCtx,
		})

		// CONTROLLER
		ans, err := controller(ctx)
		if err != nil {
			err = s.ErrorHandler(err)
			s.SerializeError(w, r, err)
			return
		}

		if reflect.TypeOf(ans) == nil {
			return
		}

		// TRANSFORM OUT
		ans, err = transformOut(r.Context(), ans)
		if err != nil {
			err = s.ErrorHandler(err)
			s.SerializeError(w, r, err)
			return
		}

		// SERIALIZATION
		err = s.Serialize(w, r, ans)
		if err != nil {
			err = s.ErrorHandler(err)
			s.SerializeError(w, r, err)
		}
	}
}
