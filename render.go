package go_ssr

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goccy/go-json"

	"github.com/rs/zerolog"
	"github.com/yunusbyrak/go-react-ssr/internal/html"
	"github.com/yunusbyrak/go-react-ssr/internal/utils"
)

// RenderConfig is the config for rendering a route
type RenderConfig struct {
	File     string
	Title    string
	MetaTags map[string]string
	Props    interface{}
	Location string
}

// RenderRoute renders a route to html
func (engine *Engine) RenderRoute(renderConfig RenderConfig) []byte {
	// routeID is the program counter of the caller
	pc, _, _, _ := runtime.Caller(1)
	routeID := fmt.Sprint(pc)

	props, err := propsToString(renderConfig.Props)
	if err != nil {
		return html.RenderError(err, routeID)
	}
	task := renderTask{
		engine:   engine,
		logger:   zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger(),
		routeID:  routeID,
		props:    props,
		filePath: filepath.ToSlash(utils.GetFullFilePath(engine.Config.FrontendDir + "/" + renderConfig.File)),
		config:   renderConfig,
		location: renderConfig.Location,
	}
	renderedHTML, css, js, err := task.Start()
	if err != nil {
		return html.RenderError(err, task.routeID)
	}
	return html.RenderHTMLString(html.Params{
		Title:      renderConfig.Title,
		MetaTags:   renderConfig.MetaTags,
		JS:         template.JS(js),
		CSS:        template.CSS(css),
		RouteID:    task.routeID,
		ServerHTML: template.HTML(renderedHTML),
	})
}

func (engine *Engine) ClientRenderRoute(renderConfig RenderConfig) []byte {
	// routeID is the program counter of the caller
	pc, _, _, _ := runtime.Caller(1)
	routeID := fmt.Sprint(pc)

	props, err := propsToString(renderConfig.Props)
	if err != nil {
		return html.RenderError(err, routeID)
	}
	task := renderTask{
		engine:   engine,
		logger:   zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger(),
		routeID:  routeID,
		props:    props,
		filePath: filepath.ToSlash(utils.GetFullFilePath(engine.Config.FrontendDir + "/" + renderConfig.File)),
		config:   renderConfig,
	}
	js, err := task.StartClient()
	if err != nil {
		return html.RenderError(err, task.routeID)
	}
	return bytes.NewBufferString(js).Bytes()
}

// Convert props to JSON string, or set to null if no props are passed
func propsToString(props interface{}) (string, error) {
	if props != nil {
		propsJSON, err := json.Marshal(props)
		return string(propsJSON), err
	}
	return "null", nil
}
