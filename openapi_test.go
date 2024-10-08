package fuego

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

type MyStruct struct {
	B string `json:"b"`
	C int    `json:"c"`
	D bool   `json:"d"`
}

type MyOutputStruct struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type testCaseForTagType[V any] struct {
	name        string
	description string
	inputType   V

	expectedTagValue     string
	expectedTagValueType *openapi3.Types
}

func Test_tagFromType(t *testing.T) {
	s := NewServer()
	type DeeplyNested *[]MyStruct
	type MoreDeeplyNested *[]DeeplyNested

	tcs := []testCaseForTagType[any]{
		{
			name:        "unknown_interface",
			description: "behind any interface",
			inputType:   *new(any),

			expectedTagValue: "unknown-interface",
		},
		{
			name:        "simple_struct",
			description: "basic struct",
			inputType:   MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "is_pointer",
			description: "",
			inputType:   &MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "is_array",
			description: "",
			inputType:   []MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "is_reference_to_array",
			description: "",
			inputType:   &[]MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "is_deeply_nested",
			description: "behind 4 pointers",
			inputType:   new(DeeplyNested),

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "5_pointers",
			description: "behind 5 pointers",
			inputType:   *new(MoreDeeplyNested),

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "6_pointers",
			description: "behind 6 pointers",
			inputType:   new(MoreDeeplyNested),

			expectedTagValue:     "default",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "7_pointers",
			description: "behind 7 pointers",
			inputType:   []*MoreDeeplyNested{},

			expectedTagValue: "default",
		},
		{
			name:        "detecting_string",
			description: "",
			inputType:   "string",

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"string"},
		},
		{
			name:        "new_string",
			description: "",
			inputType:   new(string),

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"string"},
		},
		{
			name:        "string_array",
			description: "",
			inputType:   []string{},

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "pointer_string_array",
			description: "",
			inputType:   &[]string{},

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "DataOrTemplate",
			description: "",
			inputType:   DataOrTemplate[MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "ptr to DataOrTemplate",
			description: "",
			inputType:   &DataOrTemplate[MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "DataOrTemplate of an array",
			description: "",
			inputType:   DataOrTemplate[[]MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "ptr to DataOrTemplate of an array of ptr",
			description: "",
			inputType:   &DataOrTemplate[[]*MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "ptr to DataOrTemplate of a ptr to an array",
			description: "",
			inputType:   &DataOrTemplate[*[]MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "ptr to DataOrTemplate of a ptr to an array of ptr",
			description: "",
			inputType:   &DataOrTemplate[*[]*MyStruct]{},

			expectedTagValue:     "default",
			expectedTagValueType: &openapi3.Types{"array"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tag := schemaTagFromType(s, tc.inputType)
			require.Equal(t, tc.expectedTagValue, tag.name, tc.description)
			if tc.expectedTagValueType != nil {
				require.NotNil(t, tag.Value)
				require.Equal(t, tc.expectedTagValueType, tag.Value.Type, tc.description)
			}
		})
	}
}

func TestServer_generateOpenAPI(t *testing.T) {
	s := NewServer()
	Get(s.RouterGroup(), "/", func(*ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})
	Post(s.RouterGroup(), "/post", func(*ContextWithBody[MyStruct]) ([]MyStruct, error) {
		return nil, nil
	})
	Get(s.RouterGroup(), "/post/{id}", func(*ContextNoBody) (MyOutputStruct, error) {
		return MyOutputStruct{}, nil
	})
	Post(s.RouterGroup(), "/multidimensional/post", func(*ContextWithBody[MyStruct]) ([][]MyStruct, error) {
		return nil, nil
	})
	document := s.OutputOpenAPISpec()
	require.NotNil(t, document)
	require.NotNil(t, document.Paths.Find("/"))
	require.Nil(t, document.Paths.Find("/unknown"))
	require.NotNil(t, document.Paths.Find("/post"))
	require.Equal(t, document.Paths.Find("/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Type, &openapi3.Types{"array"})
	require.Equal(t, document.Paths.Find("/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Items.Ref, "#/components/schemas/MyStruct")
	require.Equal(t, document.Paths.Find("/multidimensional/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Type, &openapi3.Types{"array"})
	require.Equal(t, document.Paths.Find("/multidimensional/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Items.Value.Type, &openapi3.Types{"array"})
	require.Equal(t, document.Paths.Find("/multidimensional/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Items.Value.Items.Ref, "#/components/schemas/MyStruct")
	require.NotNil(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200"))
	require.NotNil(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200").Value.Content["application/json"])
	require.Nil(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["unknown"])
	require.Equal(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["quantity"].Value.Type, &openapi3.Types{"integer"})

	t.Run("openapi doc is available through a route", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/swagger/openapi.json", nil)
		s.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
	})
}

func BenchmarkRoutesRegistration(b *testing.B) {
	for range b.N {
		s := NewServer(
			WithoutLogger(),
		)
		Get(s.RouterGroup(), "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})
		for j := 0; j < 100; j++ {
			Post(s.RouterGroup(), fmt.Sprintf("/post/%d", j), func(*ContextWithBody[MyStruct]) ([]MyStruct, error) {
				return nil, nil
			})
		}
		for j := 0; j < 100; j++ {
			Get(s.RouterGroup(), fmt.Sprintf("/post/{id}/%d", j), func(ContextNoBody) (MyStruct, error) {
				return MyStruct{}, nil
			})
		}
	}
}

func BenchmarkServer_generateOpenAPI(b *testing.B) {
	for range b.N {
		s := NewServer(
			WithoutLogger(),
		)
		Get(s.RouterGroup(), "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})
		for j := 0; j < 100; j++ {
			Post(s.RouterGroup(), fmt.Sprintf("/post/%d", j), func(*ContextWithBody[MyStruct]) ([]MyStruct, error) {
				return nil, nil
			})
		}
		for j := 0; j < 100; j++ {
			Get(s.RouterGroup(), fmt.Sprintf("/post/{id}/%d", j), func(ContextNoBody) (MyStruct, error) {
				return MyStruct{}, nil
			})
		}

		s.OutputOpenAPISpec()
	}
}

func TestAutoGroupTags(t *testing.T) {
	s := NewServer()
	Get(s.RouterGroup(), "/a", func(*ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	group := Group(s.RouterGroup(), "/group")
	Get(group, "/b", func(*ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	subGroup := Group(group, "/subgroup")
	Get(subGroup, "/c", func(*ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	otherGroup := Group(s.RouterGroup(), "/other")
	Get(otherGroup, "/d", func(*ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	document := s.OutputOpenAPISpec()
	require.NotNil(t, document)
	require.Nil(t, document.Paths.Find("/a").Get.Tags)
	require.Equal(t, []string{"group"}, document.Paths.Find("/group/b").Get.Tags)
	require.Equal(t, []string{"subgroup"}, document.Paths.Find("/group/subgroup/c").Get.Tags)
	require.Equal(t, []string{"other"}, document.Paths.Find("/other/d").Get.Tags)
}

func TestValidationTags(t *testing.T) {
	type MyType struct {
		Name string `json:"name" validate:"required,min=3,max=10" description:"Name of the user" example:"John"`
		Age  int    `json:"age" validate:"min=18,max=100" description:"Age of the user" example:"25"`
	}

	s := NewServer()
	Get(s.RouterGroup(), "/data", func(ContextNoBody) (MyType, error) {
		return MyType{}, nil
	})

	document := s.OutputOpenAPISpec()
	require.NotNil(t, document)
	require.NotNil(t, document.Paths.Find("/data").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["name"].Value.Description)
	require.Equal(t, "Name of the user", document.Paths.Find("/data").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["name"].Value.Description)

	myTypeValue := document.Components.Schemas["MyType"].Value
	t.Logf("myType: %+v", myTypeValue)
	t.Logf("name: %+v", myTypeValue.Properties["name"])
	t.Logf("age: %+v", myTypeValue.Properties["age"])

	require.NotNil(t, myTypeValue.Properties["name"].Value.Description)
	require.Equal(t, "John", myTypeValue.Properties["name"].Value.Example)
	require.Equal(t, "Name of the user", myTypeValue.Properties["name"].Value.Description)
	var expected *float64
	require.Equal(t, expected, myTypeValue.Properties["name"].Value.Min)
	require.Equal(t, uint64(3), myTypeValue.Properties["name"].Value.MinLength)
	require.Equal(t, uint64(10), *myTypeValue.Properties["name"].Value.MaxLength)
	require.Equal(t, float64(18.0), *myTypeValue.Properties["age"].Value.Min)
	require.Equal(t, float64(100), *myTypeValue.Properties["age"].Value.Max)
}
