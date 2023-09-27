package react_renderer

import (
	"encoding/json"
	"fmt"
	"html/template"
	"runtime"
	"strings"

	"github.com/natewong1313/go-react-ssr/config"
	"github.com/natewong1313/go-react-ssr/internal/logger"
	"github.com/natewong1313/go-react-ssr/internal/utils"
)

// Converts the given react file path to a full html page
func RenderRoute(renderConfig Config) []byte {
	// Get the program counter for the caller of this function and use that for the id
	pc, _, _, _ := runtime.Caller(1)
	routeID := fmt.Sprint(pc)
	// Props are passed to the renderer as a JSON string, or set to null if no props are passed
	props, err := getProps(renderConfig.Props)
	if err != nil {
		logger.L.Err(err).Msg("Failed to convert props to JSON")
		return renderErrorHTMLString(err)
	}
	// Get the full path of the react component file
	reactFilePath := utils.GetFullFilePath(config.C.FrontendDir + "/" + renderConfig.File)
	// Update the routeID to file map
	go updateRouteIDToReactFileMap(routeID, reactFilePath)

	// Build the client files and server html on different threads
	clientBuildChan := make(chan ClientBuildResult)
	serverBuildChan := make(chan ServerBuildResult)

	go buildForClient(reactFilePath, props, clientBuildChan)
	go buildForServer(reactFilePath, props, serverBuildChan)
	clientBuildResult := <-clientBuildChan
	serverBuildResult := <-serverBuildChan

	if clientBuildResult.Error != nil {
		logger.L.Err(clientBuildResult.Error).Msg("Error occured building file")
		return renderErrorHTMLString(clientBuildResult.Error)
	}

	if serverBuildResult.Error != nil {
		logger.L.Err(serverBuildResult.Error).Msg("Error occured building server rendered file")
		return renderErrorHTMLString(serverBuildResult.Error)
	}

	go updateParentFileDependencies(reactFilePath, clientBuildResult.Dependencies)
	// Return the rendered html
	return renderHTMLString(HTMLParams{
		Title:      renderConfig.Title,
		MetaTags:   getMetaTags(renderConfig.MetaTags),
		OGMetaTags: getOGMetaTags(renderConfig.MetaTags),
		Links:      renderConfig.Links,
		JS:         template.JS(clientBuildResult.JS),
		CSS:        template.CSS(serverBuildResult.CSS),
		RouteID:    routeID,
		ServerHTML: template.HTML(serverBuildResult.HTML),
	})
}

// Convert props to JSON string, or set to null if no props are passed
func getProps(props interface{}) (string, error) {
	if props != nil {
		propsJSON, err := json.Marshal(props)
		if err != nil {
			return "", err
		}
		return string(propsJSON), nil
	}
	return "null", nil
}

// Differentiate between meta tags and open graph meta tags

func getMetaTags(metaTags map[string]string) map[string]string {
	newMetaTags := make(map[string]string)
	for key, value := range metaTags {
		if !strings.HasPrefix(key, "og:") {
			newMetaTags[key] = value
		}
	}
	return newMetaTags
}

func getOGMetaTags(metaTags map[string]string) map[string]string {
	newMetaTags := make(map[string]string)
	for key, value := range metaTags {
		if strings.HasPrefix(key, "og:") {
			newMetaTags[key] = value
		}
	}
	return newMetaTags
}
