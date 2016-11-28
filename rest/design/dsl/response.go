package dsl

import (
	apidesign "goa.design/goa.v2/design"
	"goa.design/goa.v2/eval"
	"goa.design/goa.v2/rest/design"
)

// Response implements the response definition DSL. Response takes the name of
// the response as first parameter. goa defines all the standard HTTP status
// name as global variables so they can be readily used as response names. The
// response body data type can be specified as second argument. If a type is
// specified it overrides any type defined by in the response media type. Response
// also accepts optional arguments that correspond to the arguments defined by
// the corresponding response template (the response template with the same
// name) if there is one, see ResponseTemplate.
//
// A response may also optionally use an anonymous function as last argument to
// specify the response status code, media type and headers overriding what the
// default response or response template specifies:
//
//        Response(OK, "text/plain")           // OK response template accepts one argument:
//                                             // the media type identifier
//
//        Response(OK, BottleMedia)            // or a media type defined in the design
//
//        Response(OK, "application/vnd.bot")  // optionally referred to by identifier
//
//        Response(OK, func() {
//                Media("application/vnd.bot") // Alternatively media type is set with Media
//        })
//
//        Response(OK, BottleMedia, func() {
//                Headers(func() {             // Headers list the response HTTP headers
//                        Header("Request-Id") // Header syntax is identical to Attribute's
//                })
//        })
//
//        Response(OK, BottleMedia, func() {
//                Status(201)                  // Set response status (overrides template's)
//        })
//
//        Response("MyResponse", func() {      // Define custom response (using no template)
//                Description("This is my response")
//                Media(BottleMedia)
//                Headers(func() {
//                        Header("X-Request-Id", func() {
//                                Pattern("[a-f0-9]+")
//                        })
//                })
//                Status(200)
//        })
//
// goa defines a default response template for all the HTTP status code. The default template simply sets
// the status code. So if an action can return NotFound for example all it has to do is specify
// Response(NotFound) - there is no need to specify the status code as the default response already
// does it, in other words:
//
//	Response(NotFound)
//
// is equivalent to:
//
//	Response(NotFound, func() {
//		Status(404)
//	})
//
// goa also defines a default response template for the OK response which takes a single argument:
// the identifier of the media type used to render the response. The API DSL can define additional
// response templates or override the default OK response template using ResponseTemplate.
//
// The media type identifier specified in a response definition via the Media function can be
// "generic" such as "text/plain" or "application/json" or can correspond to the identifier of a
// media type defined in the API DSL. In this latter case goa uses the media type definition to
// generate helper response methods. These methods know how to render the views defined on the media
// type and run the validations defined in the media type during rendering.
func Response(name string, paramsAndDSL ...interface{}) {
	switch def := eval.Current().(type) {
	case *design.ActionExpr:
		if def.Response(name) != nil {
			eval.ReportError("response %s is defined twice", name)
			return
		}
		if resp := executeResponseDSL(name, paramsAndDSL...); resp != nil {
			if resp.Status == 200 && resp.MediaType == "" {
				resp.MediaType = def.Resource.MediaType
			}
			resp.Parent = def
			def.Responses = append(def.Responses, resp)
		}

	case *design.ResourceExpr:
		if def.Response(name) != nil {
			eval.ReportError("response %s is defined twice", name)
			return
		}
		if resp := executeResponseDSL(name, paramsAndDSL...); resp != nil {
			if resp.Status == 200 && resp.MediaType == "" {
				resp.MediaType = def.MediaType
			}
			resp.Parent = def
			def.Responses = append(def.Responses, resp)
		}

	default:
		eval.IncompatibleDSL()
	}
}

// Status sets the Response status.
func Status(status int) {
	res, ok := eval.Current().(*design.HTTPResponseExpr)
	if !ok {
		eval.IncompatibleDSL()
		return
	}
	res.Status = status
}

func executeResponseDSL(name string, paramsAndDSL ...interface{}) *design.HTTPResponseExpr {
	var params []string
	var dsl func()
	var ok bool
	var dt apidesign.DataType
	if len(paramsAndDSL) > 0 {
		d := paramsAndDSL[len(paramsAndDSL)-1]
		if dsl, ok = d.(func()); ok {
			paramsAndDSL = paramsAndDSL[:len(paramsAndDSL)-1]
		}
		if len(paramsAndDSL) > 0 {
			t := paramsAndDSL[0]
			if dt, ok = t.(apidesign.DataType); ok {
				paramsAndDSL = paramsAndDSL[1:]
			}
		}
		params = make([]string, len(paramsAndDSL))
		for i, p := range paramsAndDSL {
			params[i], ok = p.(string)
			if !ok {
				eval.ReportError("invalid response template parameter %#v, must be a string", p)
				return nil
			}
		}
	}
	var resp *design.HTTPResponseExpr
	if len(params) > 0 {
		eval.ReportError("no response template named %#v", name)
		return nil
	}
	if ar := design.Root.Response(name); ar != nil {
		resp = ar.Dup()
	} else if ar := design.Root.DefaultResponse(name); ar != nil {
		resp = ar.Dup()
		resp.Standard = true
	} else {
		resp = &design.HTTPResponseExpr{Name: name}
	}
	if dsl != nil {
		if !eval.Execute(dsl, resp) {
			return nil
		}
		resp.Standard = false
	}
	if dt != nil {
		if mt, ok := dt.(*apidesign.MediaTypeExpr); ok {
			resp.MediaType = mt.Identifier
		}
		resp.Type = dt
		resp.Standard = false
	}
	return resp
}
