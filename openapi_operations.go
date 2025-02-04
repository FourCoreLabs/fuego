package fuego

import (
	"log/slog"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"
)

type ParamType string

const (
	QueryParamType  ParamType = openapi3.ParameterInQuery
	HeaderParamType ParamType = openapi3.ParameterInHeader
	CookieParamType ParamType = openapi3.ParameterInCookie
)

type OpenAPIParam struct {
	Name        string
	Description string
	Type        ParamType

	opts []func(*openapi3.Parameter)
}

func WithRequiredParam() func(opt *openapi3.Parameter) {
	return func(opt *openapi3.Parameter) {
		opt.Required = true
	}
}

func WithExample(example any) func(opt *openapi3.Parameter) {
	return func(opt *openapi3.Parameter) {
		opt.Example = example
	}
}

func WithExamples(examples map[string]any) func(opt *openapi3.Parameter) {
	return func(opt *openapi3.Parameter) {
		if opt.Examples == nil {
			opt.Examples = make(openapi3.Examples)
		}

		for k, v := range examples {
			ref := openapi3.ExampleRef{
				Value: openapi3.NewExample(v),
			}

			opt.Examples[k] = &ref
		}
	}
}

func WithSchema(schema *openapi3.Schema) func(opt *openapi3.Parameter) {
	return func(opt *openapi3.Parameter) {
		s := &schema
		ref := openapi3.SchemaRef{
			Value: *s,
		}

		opt.Schema = &ref
	}
}

func WithExplode() func(opt *openapi3.Parameter) {
	return func(opt *openapi3.Parameter) {
		t := true
		opt.Explode = &t
	}
}

func WithAllowReserved() func(opt *openapi3.Parameter) {
	return func(opt *openapi3.Parameter) {
		opt.AllowReserved = true
	}
}

// Overrides the description for the route.
func (r Route) Description(description string) Route {
	r.Operation.Description = description
	return r
}

// Overrides the summary for the route.
func (r Route) Summary(summary string) Route {
	r.Operation.Summary = summary
	return r
}

// Overrides the operationID for the route.
func (r Route) OperationID(operationID string) Route {
	r.Operation.OperationID = operationID
	return r
}

// Param registers a parameter for the route.
// The paramType can be "query", "header" or "cookie" as defined in [ParamType].
// [Cookie], [Header], [QueryParam] are shortcuts for Param.
func (r Route) Param(paramType ParamType, name, description string, opts ...func(*openapi3.Parameter)) Route {
	openapiParam := openapi3.NewQueryParameter(name)
	openapiParam.Description = description
	openapiParam.Schema = openapi3.NewStringSchema().NewRef()
	openapiParam.In = string(paramType)

	for _, opt := range opts {
		opt(openapiParam)
	}

	r.Operation.AddParameter(openapiParam)

	return r
}

// Header registers a header parameter for the route.
func (r Route) Header(name, description string, opts ...func(*openapi3.Parameter)) Route {
	r.Param(HeaderParamType, name, description, opts...)
	return r
}

// Cookie registers a cookie parameter for the route.
func (r Route) Cookie(name, description string, opts ...func(*openapi3.Parameter)) Route {
	r.Param(CookieParamType, name, description, opts...)
	return r
}

// QueryParam registers a query parameter for the route.
func (r Route) Query(name, description string, opts ...func(*openapi3.Parameter)) Route {
	r.Param(QueryParamType, name, description, opts...)
	return r
}

// Replace the tags for the route.
// By default, the tag is the type of the response body.
func (r Route) Tags(tags ...string) Route {
	r.Operation.Tags = tags
	return r
}

// AddTags adds tags to the route.
func (r Route) AddTags(tags ...string) Route {
	r.Operation.Tags = append(r.Operation.Tags, tags...)
	return r
}

// AddError adds an error to the route.
func (r Route) AddError(code int, errType any, description string, contentType ...string) Route {
	if len(contentType) == 0 {
		contentType = append(contentType, "application/json")
	}

	schema := Schema{
		Type:        errType,
		Description: description,
		ContentType: contentType,
	}

	r.Errors = append(r.Errors, openAPIError{
		Code:   code,
		Schema: schema,
	})

	return r
}

// RemoveTags removes tags from the route.
func (r Route) RemoveTags(tags ...string) Route {
	for _, tag := range tags {
		for i, t := range r.Operation.Tags {
			if t == tag {
				r.Operation.Tags = slices.Delete(r.Operation.Tags, i, i+1)
				break
			}
		}
	}
	return r
}

func (r Route) Deprecated() Route {
	r.Operation.Deprecated = true
	return r
}

func (r Route) WithRequest(reqType any, contentType ...string) Route {
	if len(contentType) == 0 {
		contentType = append(contentType, "application/json")
	}

	r.Request = Schema{
		Type:        reqType,
		ContentType: contentType,
	}

	return r
}

func (r Route) RequestDescription(desc string) Route {
	r.Request.Description = desc
	return r
}

func (r Route) ResponseDescription(desc string) Route {
	r.Response.Description = desc
	return r
}

func (r Route) WithResponse(resType any, contentType ...string) Route {
	if len(contentType) == 0 {
		contentType = append(contentType, "application/json")
	}

	r.Response = Schema{
		Type:        resType,
		ContentType: contentType,
	}

	return r
}

func (r Route) RequestContentType(contentType string) Route {
	r.Request.ContentType = []string{contentType}
	return r
}

func (r Route) ResponseContentType(contentType string) Route {
	r.Response.ContentType = []string{contentType}
	return r
}

func (r Route) With(opts func(Route) Route) Route {
	return opts(r)
}

func (r Route) Build() {
	for _, opt := range r.Group.routeCfg {
		r = opt(r)
	}

	if r.Group.DisableOpenapi || r.Method == "" || r.All {
		return
	}

	var err error
	r.Operation, err = RegisterOpenAPIOperation(r.Group, r)
	if err != nil {
		slog.Warn("error documenting openapi operation", "error", err)
	}

	if r.Operation.OperationID == "" {
		r.Operation.OperationID = r.Method + "_" + r.Path
	}
}
