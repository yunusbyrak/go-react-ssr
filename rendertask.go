package go_ssr

import (
	"encoding/base64"
	"fmt"

	"github.com/buke/quickjs-go"
	"github.com/rs/zerolog"
	"github.com/yunusbyrak/go-react-ssr/internal/reactbuilder"
	v8 "rogchap.com/v8go"
)

type renderTask struct {
	engine                *Engine
	logger                zerolog.Logger
	routeID               string
	filePath              string
	props                 string
	config                RenderConfig
	location              string
	serverRenderResult    chan serverRenderResult
	clientRenderResult    chan clientRenderResult
	clientRawRenderResult chan clientRenderResult
}

type serverRenderResult struct {
	html string
	css  string
	err  error
}

type clientRenderResult struct {
	js           string
	dependencies []string
	err          error
}

// Start starts the render task, returns the rendered html, css, and js for hydration
func (rt *renderTask) Start() (string, string, string, error) {
	rt.serverRenderResult = make(chan serverRenderResult)
	rt.clientRenderResult = make(chan clientRenderResult)
	// Assigns the parent file to the routeID so that the cache can be invalidated when the parent file changes
	rt.engine.CacheManager.SetParentFile(rt.routeID, rt.filePath)

	// Render for server and client concurrently
	go rt.doRender("server")
	go rt.doRender("client")

	// Wait for both to finish
	srResult := <-rt.serverRenderResult
	if srResult.err != nil {
		rt.logger.Error().Err(srResult.err).Msg("Failed to build for server")
		return "", "", "", srResult.err
	}
	crResult := <-rt.clientRenderResult
	if crResult.err != nil {
		rt.logger.Error().Err(crResult.err).Msg("Failed to build for client")
		return "", "", "", crResult.err
	}

	// Set the parent file dependencies so that the cache can be invalidated a dependency changes
	go rt.engine.CacheManager.SetParentFileDependencies(rt.filePath, crResult.dependencies)
	return srResult.html, srResult.css, crResult.js, nil
}

func (rt *renderTask) StartClient() (string, error) {
	rt.clientRawRenderResult = make(chan clientRenderResult)
	// Assigns the parent file to the routeID so that the cache can be invalidated when the parent file changes
	rt.engine.CacheManager.SetParentFile(rt.routeID, rt.filePath)

	fmt.Printf("hello from start client\n")
	// Render for server and client concurrently
	go rt.doRender("client_raw")

	crResult := <-rt.clientRawRenderResult
	if crResult.err != nil {
		rt.logger.Error().Err(crResult.err).Msg("Failed to build for client")
		return "", crResult.err
	}

	// Set the parent file dependencies so that the cache can be invalidated a dependency changes
	go rt.engine.CacheManager.SetParentFileDependencies(rt.filePath, crResult.dependencies)
	return crResult.js, nil
}

func (rt *renderTask) doRender(buildType string) {
	// Check if the build is in the cache
	fmt.Printf("hello from doRender: %s", buildType)
	build, buildFound := rt.getBuildFromCache(buildType)
	if !buildFound {
		// Build the file if it's not in the cache
		newBuild, err := rt.buildFile(buildType)
		if err != nil {
			rt.handleBuildError(err, buildType)
			return
		}
		rt.updateBuildCache(newBuild, buildType)
		build = newBuild
	}
	// JS is built without props so that the props can be injected into cached JS builds

	js := injectProps(build.JS, rt.props)
	switch buildType {
	case "server":

		// Then call that file with node to get the rendered HTML
		renderedHTML, err := renderReactToHTML(js)
		rt.serverRenderResult <- serverRenderResult{html: renderedHTML, css: build.CSS, err: err}
	case "client":
		rt.clientRenderResult <- clientRenderResult{js: js, dependencies: build.Dependencies}
	default:
		rt.clientRawRenderResult <- clientRenderResult{js: build.JS, dependencies: build.Dependencies}
	}
}

// getBuild returns the build from the cache if it exists
func (rt *renderTask) getBuildFromCache(buildType string) (reactbuilder.BuildResult, bool) {
	switch buildType {
	case "server":
		return rt.engine.CacheManager.GetServerBuild(rt.filePath)
	case "client":
		return rt.engine.CacheManager.GetClientBuild(rt.filePath)
	default:
		return rt.engine.CacheManager.GetClientRawBuild(rt.filePath)
	}
}

// buildFile gets the contents of the file to be built and builds it with reactbuilder
func (rt *renderTask) buildFile(buildType string) (reactbuilder.BuildResult, error) {
	buildContents, err := rt.getBuildContents(buildType)
	if err != nil {
		return reactbuilder.BuildResult{}, err
	}
	fmt.Printf("build type: %s\n", buildType)
	switch buildType {
	case "server":
		return reactbuilder.BuildServer(buildContents, rt.engine.Config.FrontendDir, rt.engine.Config.AssetRoute)
	case "client":
		return reactbuilder.BuildClient(buildContents, rt.engine.Config.FrontendDir, rt.engine.Config.AssetRoute)
	default:
		return reactbuilder.BuildClientRaw(buildContents, rt.engine.Config.FrontendDir, rt.engine.Config.AssetRoute)
	}
}

// getBuildContents gets the required imports based on the config and returns the contents to be built with reactbuilder
func (rt *renderTask) getBuildContents(buildType string) (string, error) {
	var imports []string
	if rt.engine.CachedLayoutCSSFilePath != "" && buildType != "client_raw" {
		imports = append(imports, fmt.Sprintf(`import "%s";`, rt.engine.CachedLayoutCSSFilePath))
	}
	if rt.engine.Config.LayoutFilePath != "" && buildType != "client_raw" {
		imports = append(imports, fmt.Sprintf(`import Layout from "%s";`, rt.engine.Config.LayoutFilePath))
	}
	switch buildType {
	case "server":
		return reactbuilder.GenerateServerBuildContents(imports, rt.filePath, rt.engine.Config.LayoutFilePath != "", rt.location)
	case "client":
		return reactbuilder.GenerateClientBuildContents(imports, rt.filePath, rt.engine.Config.LayoutFilePath != "")
	default:
		return reactbuilder.GenerateRawClientBuildContents(imports, rt.filePath, rt.engine.Config.LayoutFilePath != "")
	}
}

// handleBuildError handles the error from building the file and sends it to the appropriate channel
func (rt *renderTask) handleBuildError(err error, buildType string) {
	switch buildType {
	case "server":
		rt.serverRenderResult <- serverRenderResult{err: err}
	case "client":
		rt.clientRenderResult <- clientRenderResult{err: err}
	default:
		rt.clientRawRenderResult <- clientRenderResult{err: err}
	}
}

// updateBuildCache updates the cache with the new build
func (rt *renderTask) updateBuildCache(build reactbuilder.BuildResult, buildType string) {
	switch buildType {
	case "server":
		rt.engine.CacheManager.SetServerBuild(rt.filePath, build)
	case "client":
		rt.engine.CacheManager.SetClientBuild(rt.filePath, build)
	default:
		rt.engine.CacheManager.SetClientRawBuild(rt.filePath, build)
	}
}

// injectProps injects the props into the already compiled JS
func injectProps(compiledJS, props string) string {
	return fmt.Sprintf(`var props = %s; %s`, props, compiledJS)
}

// renderReactToHTML uses node to execute the server js file which outputs the rendered HTML
func renderReactToHTMLNew(js string) (string, error) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()
	globals := ctx.Globals()
	globals.Set("atob", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		l, _ := base64.RawStdEncoding.DecodeString(args[0].String())
		return ctx.String(string(l))
	}))
	res, err := ctx.Eval(js)
	if err != nil {
		return "", err
	}
	return res.String(), nil
}

func renderReactToHTML(js string) (string, error) {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext()
	defer ctx.Close()
	res, err := ctx.RunScript(js, "render.js")
	if err != nil {
		return "", err
	}

	return res.String(), nil
}
