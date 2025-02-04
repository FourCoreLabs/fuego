package fuego

import (
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
)

func NewOpenApiSpec() openapi3.T {
	info := &openapi3.Info{
		Title:       "OpenAPI",
		Description: openapiDescription,
		Version:     "0.0.1",
	}
	spec := openapi3.T{
		OpenAPI:  "3.1.0",
		Info:     info,
		Paths:    &openapi3.Paths{},
		Servers:  []*openapi3.Server{},
		Security: openapi3.SecurityRequirements{},
		Components: &openapi3.Components{
			Schemas:       make(map[string]*openapi3.SchemaRef),
			RequestBodies: make(map[string]*openapi3.RequestBodyRef),
			Responses:     make(map[string]*openapi3.ResponseRef),
		},
	}
	return spec
}

// Hide prevents the routes in this server or group from being included in the OpenAPI spec.
func (s *RouterGroup) Hide() *RouterGroup {
	s.DisableOpenapi = true
	return s
}

// Show allows displaying the routes. Activated by default so useless in most cases,
// but this can be useful if you deactivated the parent group.
func (s *RouterGroup) Show() *RouterGroup {
	s.DisableOpenapi = false
	return s
}

// RouteConfig will apply config to each route in that group
func (s *RouterGroup) RouteConfig(cfg func(Route) Route) {
	s.routeCfg = append(s.routeCfg, cfg)
}

// OutputOpenAPISpec takes the OpenAPI spec and outputs it to a JSON file and/or serves it on a URL.
// Also serves a Swagger UI.
// To modify its behavior, use the [WithOpenAPIConfig] option.
func (s *Server) OutputOpenAPISpec() openapi3.T {
	// Validate
	err := s.OpenApiSpec.Validate(context.Background())
	if err != nil {
		slog.Error("Error validating spec", "error", err)
	}

	return s.OpenApiSpec
}

func (s *Server) MarshalSpec(prettyFormatJSON bool) ([]byte, error) {
	if prettyFormatJSON {
		return json.MarshalIndent(s.OpenApiSpec, "", "	")
	}
	return json.Marshal(s.OpenApiSpec)
}

var generator = openapi3gen.NewGenerator(
	openapi3gen.UseAllExportedFields(),
)

// RegisterOpenAPIOperation registers an OpenAPI operation.
func RegisterOpenAPIOperation(group *RouterGroup, route Route) (*openapi3.Operation, error) {
	if route.Operation == nil {
		route.Operation = openapi3.NewOperation()
	}

	if group.tags != nil {
		route.Operation.Tags = append(route.Operation.Tags, group.tags...)
	}

	// Tags
	if !group.server.disableAutoGroupTags && group.groupTag != "" {
		route.Operation.Tags = append(route.Operation.Tags, group.groupTag)
	}

	for _, param := range group.params {
		route.Param(param.Type, param.Name, param.Description, param.opts...)
	}

	// Request Body
	if route.Operation.RequestBody == nil && route.Request.Type != nil {
		bodyTag := schemaTagFromType(group.server, route.Request.Type)

		if bodyTag.name != "unknown-interface" {
			requestBody := newRequestBody(bodyTag, route.Request)
			group.server.OpenApiSpec.Components.RequestBodies[bodyTag.name] = &openapi3.RequestBodyRef{
				Value: requestBody,
			}

			// add request body to operation
			route.Operation.RequestBody = &openapi3.RequestBodyRef{
				Ref:   "#/components/requestBodies/" + bodyTag.name,
				Value: requestBody,
			}
		}
	}

	// Response - globals
	for _, openAPIGlobalResponse := range group.server.globalOpenAPIResponses {
		addResponse(group.server, route.Operation, openAPIGlobalResponse.Code, openAPIGlobalResponse.Schema)
	}

	// Response - locals
	for _, openAPIErrors := range route.Errors {
		addResponse(group.server, route.Operation, openAPIErrors.Code, openAPIErrors.Schema)
	}

	// Response - 200
	if route.Response.Type != nil {
		addResponse(group.server, route.Operation, 200, route.Response)
	}

	for _, pathParam := range parseGinPathParams(route.Path) {
		pathParam := strings.TrimPrefix(pathParam, ":")
		if pathParam == "" {
			continue
		}

		parameter := openapi3.NewPathParameter(pathParam)

		parameter.Schema = openapi3.NewStringSchema().NewRef()
		if pathParam[0] == '*' {
			parameter.Description += " (might contain slashes)"
		}

		route.Operation.AddParameter(parameter)
	}

	group.server.OpenApiSpec.AddOperation(convertGinPathToStdPath(route.Path), route.Method, route.Operation)

	return route.Operation, nil
}

func addResponse(s *Server, operation *openapi3.Operation, code int, schema Schema) {
	responseSchema := schemaTagFromType(s, schema.Type)

	// add default type to content type
	content := openapi3.NewContentWithSchemaRef(&responseSchema.SchemaRef, schema.ContentType)

	response := openapi3.NewResponse().
		WithDescription(schema.Description).
		WithContent(content)

	operation.AddResponse(code, response)
}

func newRequestBody(tag schemaTag, schema Schema) *openapi3.RequestBody {
	content := openapi3.NewContentWithSchemaRef(&tag.SchemaRef, schema.ContentType)
	return openapi3.NewRequestBody().
		WithRequired(true).
		WithDescription(schema.Description).
		WithContent(content)
}

// schemaTag is a struct that holds the name of the struct and the associated openapi3.SchemaRef
type schemaTag struct {
	openapi3.SchemaRef
	name string
}

func schemaTagFromType(s *Server, v any) schemaTag {
	if v == nil {
		// ensure we add unknown-interface to our schemas
		schema := s.getOrCreateSchema("unknown-interface", struct{}{})
		return schemaTag{
			name: "unknown-interface",
			SchemaRef: openapi3.SchemaRef{
				Ref:   "#/components/schemas/unknown-interface",
				Value: schema,
			},
		}
	}

	return dive(s, reflect.TypeOf(v), schemaTag{}, 5)
}

// dive returns a schemaTag which includes the generated openapi3.SchemaRef and
// the name of the struct being passed in.
// If the type is a pointer, map, channel, function, or unsafe pointer,
// it will dive into the type and return the name of the type it points to.
// If the type is a slice or array type it will dive into the type as well as
// build and openapi3.Schema where Type is array and Ref is set to the proper
// components Schema
func dive(s *Server, t reflect.Type, tag schemaTag, maxDepth int) schemaTag {
	if maxDepth == 0 {
		return schemaTag{
			name: "default",
			SchemaRef: openapi3.SchemaRef{
				Ref: "#/components/schemas/default",
			},
		}
	}

	switch t.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return dive(s, t.Elem(), tag, maxDepth-1)

	case reflect.Slice, reflect.Array:
		item := dive(s, t.Elem(), tag, maxDepth-1)
		tag.name = item.name
		tag.Value = openapi3.NewArraySchema()
		tag.Value.Items = &item.SchemaRef
		return tag

	default:
		tag.name = getName(t)
		if t.Kind() == reflect.Struct && strings.HasPrefix(tag.name, "DataOrTemplate") {
			return dive(s, t.Field(0).Type, tag, maxDepth-1)
		}
		tag.Ref = "#/components/schemas/" + tag.name
		tag.Value = s.getOrCreateSchema(tag.name, reflect.New(t).Interface())

		return tag
	}
}

// getName remove generic path from name if any present
func getName(t reflect.Type) string {
	v := reflect.New(t).Interface()
	name := t.Name()

	if namer, ok := v.(OpenAPINamer); ok {
		if n := namer.OpenApiName(); n != "" {
			name = n
		}
	}

	if name == "" {
		return t.Name()
	}

	if len(name) == 0 || name[len(name)-1] == ']' {
		generic := ""
		name, generic, _ = strings.Cut(name[:len(name)-1], "[")

		sep := strings.LastIndexAny(generic, "/.")
		if sep != -1 {
			generic = generic[sep+1:]
		}

		name = name + " " + generic
	}

	builder := strings.Builder{}
	builder.Grow(len(name))
	for _, field := range strings.Fields(name) {
		builder.WriteString(strings.Title(field))
	}

	return builder.String()
}

// getOrCreateSchema is used to get a schema from the OpenAPI spec.
// If the schema does not exist, it will create a new schema and add it to the OpenAPI spec.
func (s *Server) getOrCreateSchema(key string, v any) *openapi3.Schema {
	schemaRef, ok := s.OpenApiSpec.Components.Schemas[key]
	if !ok {
		schemaRef = s.createSchema(key, v)
	}
	return schemaRef.Value
}

// createSchema is used to create a new schema and add it to the OpenAPI spec.
// Relies on the openapi3gen package to generate the schema, and adds custom struct tags.
func (s *Server) createSchema(key string, v any) *openapi3.SchemaRef {
	schemaRef, err := generator.NewSchemaRefForValue(v, s.OpenApiSpec.Components.Schemas)
	if err != nil {
		slog.Error("Error generating schema", "key", key, "error", err)
	}

	descriptionable, ok := v.(OpenAPIDescriptioner)
	if ok {
		schemaRef.Value.Description = descriptionable.Description()
	}

	s.parseStructTags(reflect.TypeOf(v), schemaRef)

	s.OpenApiSpec.Components.Schemas[key] = schemaRef

	return schemaRef
}

// parseStructTags parses struct tags and modifies the schema accordingly.
// t must be a struct type.
// It adds the following struct tags (tag => OpenAPI schema field):
// - description => description
// - example => example
// - json => nullable (if contains omitempty)
// - validate:
//   - required => required
//   - min=1 => min=1 (for integers)
//   - min=1 => minLength=1 (for strings)
//   - max=100 => max=100 (for integers)
//   - max=100 => maxLength=100 (for strings)
func (s *Server) parseStructTags(t reflect.Type, schemaRef *openapi3.SchemaRef) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	for i := range t.NumField() {
		field := t.Field(i)
		if field.Anonymous {
			s.parseStructTags(field.Type, schemaRef)
			continue
		}

		parseStructTagFields(field, schemaRef)
	}
}

func parseStructTagFields(field reflect.StructField, schemaRef *openapi3.SchemaRef) {
	jsonFieldName := field.Tag.Get("json")
	jsonFieldName = strings.Split(jsonFieldName, ",")[0] // remove omitempty, etc
	if jsonFieldName == "-" {
		return
	}
	if jsonFieldName == "" {
		jsonFieldName = field.Name
	}

	property := schemaRef.Value.Properties[jsonFieldName]
	if property == nil {
		slog.Warn("Property not found in schema", "property", jsonFieldName)
		return
	}

	propertyCopy := *property
	propertyValue := *propertyCopy.Value

	// Example
	example, ok := field.Tag.Lookup("example")
	if ok {
		propertyValue.Example = example
		if propertyValue.Type.Is(openapi3.TypeInteger) {
			exNum, err := strconv.Atoi(example)
			if err != nil {
				slog.Warn("Example might be incorrect (should be integer)", "error", err)
			}
			propertyValue.Example = exNum
		}
	}

	// Validation
	validateTag, ok := field.Tag.Lookup("validate")
	validateTags := strings.Split(validateTag, ",")
	if ok && slices.Contains(validateTags, "required") {
		schemaRef.Value.Required = append(schemaRef.Value.Required, jsonFieldName)
	}
	for _, validateTag := range validateTags {
		if strings.HasPrefix(validateTag, "min=") {
			min, err := strconv.Atoi(strings.Split(validateTag, "=")[1])
			if err != nil {
				slog.Warn("Min might be incorrect (should be integer)", "error", err)
			}

			if propertyValue.Type.Is(openapi3.TypeInteger) {
				minPtr := float64(min)
				propertyValue.Min = &minPtr
			} else if propertyValue.Type.Is(openapi3.TypeString) {
				propertyValue.MinLength = uint64(min)
			}
		}

		if strings.HasPrefix(validateTag, "max=") {
			max, err := strconv.Atoi(strings.Split(validateTag, "=")[1])
			if err != nil {
				slog.Warn("Max might be incorrect (should be integer)", "error", err)
			}
			if propertyValue.Type.Is(openapi3.TypeInteger) {
				maxPtr := float64(max)
				propertyValue.Max = &maxPtr
			} else if propertyValue.Type.Is(openapi3.TypeString) {
				maxPtr := uint64(max)
				propertyValue.MaxLength = &maxPtr
			}
		}
	}

	// Description
	description, ok := field.Tag.Lookup("description")
	if ok {
		propertyValue.Description = description
	}
	jsonTag, ok := field.Tag.Lookup("json")
	if ok {
		if strings.Contains(jsonTag, ",omitempty") {
			propertyValue.Nullable = true
		}
	}

	propertyCopy.Value = &propertyValue
	schemaRef.Value.Properties[jsonFieldName] = &propertyCopy
}

type OpenAPIDescriptioner interface {
	Description() string
}

type OpenAPINamer interface {
	OpenApiName() string
}
