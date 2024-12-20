package fuego

import (
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
func (r Route[ResponseBody, RequestBody]) Description(description string) Route[ResponseBody, RequestBody] {
	r.Operation.Description = description
	r = r.addPermissionDesc()
	return r
}

func (r Route[ResponseBody, RequestBody]) addPermissionDesc() Route[ResponseBody, RequestBody] {
	desc := ""
	if r.mainRouter.PermissionFunc != nil {
		desc += "Permissions: "
		for _, perm := range r.mainRouter.PermissionFunc(stdToGinPath(r.Path), r.Method) {
			desc += "```" + perm + "``` "
		}
	}

	r.Operation.Description += "\n\n" + desc
	return r
}

// Overrides the summary for the route.
func (r Route[ResponseBody, RequestBody]) Summary(summary string) Route[ResponseBody, RequestBody] {
	r.Operation.Summary = summary
	return r
}

// Overrides the operationID for the route.
func (r Route[ResponseBody, RequestBody]) OperationID(operationID string) Route[ResponseBody, RequestBody] {
	r.Operation.OperationID = operationID
	return r
}

// Param registers a parameter for the route.
// The paramType can be "query", "header" or "cookie" as defined in [ParamType].
// [Cookie], [Header], [QueryParam] are shortcuts for Param.
func (r Route[ResponseBody, RequestBody]) Param(paramType ParamType, name, description string, opts ...func(*openapi3.Parameter)) Route[ResponseBody, RequestBody] {
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
func (r Route[ResponseBody, RequestBody]) Header(name, description string, opts ...func(*openapi3.Parameter)) Route[ResponseBody, RequestBody] {
	r.Param(HeaderParamType, name, description, opts...)
	return r
}

// Cookie registers a cookie parameter for the route.
func (r Route[ResponseBody, RequestBody]) Cookie(name, description string, opts ...func(*openapi3.Parameter)) Route[ResponseBody, RequestBody] {
	r.Param(CookieParamType, name, description, opts...)
	return r
}

// QueryParam registers a query parameter for the route.
func (r Route[ResponseBody, RequestBody]) Query(name, description string, opts ...func(*openapi3.Parameter)) Route[ResponseBody, RequestBody] {
	r.Param(QueryParamType, name, description, opts...)
	return r
}

// Replace the tags for the route.
// By default, the tag is the type of the response body.
func (r Route[ResponseBody, RequestBody]) Tags(tags ...string) Route[ResponseBody, RequestBody] {
	r.Operation.Tags = tags
	return r
}

// Replace the available request Content-Types for the route.
// By default, the request Content-Types is `application/json`.
func (r Route[ResponseBody, RequestBody]) RequestContentType(consumes ...string) Route[ResponseBody, RequestBody] {
	bodyTag := schemaTagFromType(r.mainRouter, *new(RequestBody))

	if bodyTag.name != "unknown-interface" {
		requestBody := newRequestBody[RequestBody](bodyTag, consumes)

		// set just Value as we do not want to reference
		// a global requestBody
		r.Operation.RequestBody = &openapi3.RequestBodyRef{
			Value: requestBody,
		}
	}
	return r
}

// AddTags adds tags to the route.
func (r Route[ResponseBody, RequestBody]) AddTags(tags ...string) Route[ResponseBody, RequestBody] {
	r.Operation.Tags = append(r.Operation.Tags, tags...)
	return r
}

// AddError adds an error to the route.
func (r Route[ResponseBody, RequestBody]) AddError(code int, description string, errorType ...any) Route[ResponseBody, RequestBody] {
	addResponse(r.mainRouter, r.Operation, code, description, errorType...)
	return r
}

// add request description
func (r Route[ResponseBody, RequestBody]) RequestDescription(desc string) Route[ResponseBody, RequestBody] {
	r.Operation.RequestBody.Value.Description = desc
	return r
}

func addResponse(s *Server, operation *openapi3.Operation, code int, description string, errorType ...any) {
	var responseSchema schemaTag

	if len(errorType) > 0 {
		responseSchema = schemaTagFromType(s, errorType[0])
	} else {
		responseSchema = schemaTagFromType(s, HTTPError{})
	}
	content := openapi3.NewContentWithSchemaRef(&responseSchema.SchemaRef, []string{"application/json"})

	response := openapi3.NewResponse().
		WithDescription(description).
		WithContent(content)

	operation.AddResponse(code, response)
}

// openAPIError describes a response error in the OpenAPI spec.
type openAPIError struct {
	Code        int
	Description string
	ErrorType   any
}

// RemoveTags removes tags from the route.
func (r Route[ResponseBody, RequestBody]) RemoveTags(tags ...string) Route[ResponseBody, RequestBody] {
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

func (r Route[ResponseBody, RequestBody]) Deprecated() Route[ResponseBody, RequestBody] {
	r.Operation.Deprecated = true
	return r
}
