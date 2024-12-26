package main

import (
	"net/http"

	"github.com/fourcorelabs/fuego"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

func fuegoRouter(ctx fuego.ContextNoBody) (any, error) {
	return "Hello, World!", nil
}

func openAPIRouter() *fuego.Server {
	spec := fuego.NewOpenApiSpec()
	spec.Info = &openapi3.Info{
		Title:          "FourCore ATTACK",
		Description:    "REST API for the FourCore ATTACK Adversary Emulation Platform",
		Version:        "1.0",
		TermsOfService: "https://fourcore.io/terms",
		Contact: &openapi3.Contact{
			Name:  "FourCore",
			Email: "support@fourcore.io",
		},
	}

	spec.Components.SecuritySchemes = openapi3.SecuritySchemes{
		"BearerAuth": &openapi3.SecuritySchemeRef{
			Value: &openapi3.SecurityScheme{
				Type:        "http",
				Scheme:      "bearer",
				Description: "Generate an API token from FourCore ATTACK Dashboard",
			},
		},
	}

	spec.Security = *openapi3.NewSecurityRequirements().With(openapi3.SecurityRequirement{
		"BearerAuth": []string{},
	})

	spec.AddServer(&openapi3.Server{
		URL:         "localhost:8080",
		Description: "FourCore ATTACK API Endpoint",
	})

	s := fuego.NewServer(
		// fuego.WithoutLogger(),
		fuego.WithSerializer(fuego.Send),
		fuego.WithGlobalResponseTypes(http.StatusBadRequest, "Bad Request _(validation or deserialization error)_", fuego.HTTPError{}),
		fuego.WithGlobalResponseTypes(http.StatusInternalServerError, "Internal Server Error", fuego.HTTPError{}),
	)

	s.OpenApiSpec = spec
	return s
}

func main() {
	s := openAPIRouter()

	noAuthRouteNoDoc := s.RouterGroup().Group("", fuego.WithoutTag()).Hide()
	fuego.GetGin(noAuthRouteNoDoc, "/openapi.json", func(ctx *gin.Context) {
		fuego.SendJSON(ctx.Writer, ctx.Request, s.OpenApiSpec)
	})

	fuego.GetGin(s.RouterGroup(), "/api/docs", func(ctx *gin.Context) {
		fuego.DefaultOpenAPIHandler("/openapi.json").ServeHTTP(ctx.Writer, ctx.Request)
	}).Build()

	fuego.Get(s.RouterGroup(), "/", fuegoRouter).Summary("hello world").Description("my world is here").WithRequest("hello world").WithResponse("my world").Build()
	s.Run(":8080")
}
