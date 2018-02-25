package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/mattermost/mattermost-server/plugin/rpcplugin"
)

const (
	// Triggers used to define slash commands
	triggerGif  = "gif"
	triggerGifs = "gifs"
)

// GiphyPlugin is a Mattermost plugin that adds a /gif slash command
// to display a GIF based on user keywords.
type GiphyPlugin struct {
	api           plugin.API
	configuration atomic.Value
	gifProvider   gifProvider
	enabled       bool
}

// GiphyPluginConfiguration contains Mattermost GiphyPlugin configuration settings
type GiphyPluginConfiguration struct {
	Rating           string
	Language         string
	Rendition        string
	ResponseTemplate string
	APIKey           string
}

// OnActivate register the plugin commands
func (p *GiphyPlugin) OnActivate(api plugin.API) error {
	p.api = api
	p.enabled = true
	err := api.RegisterCommand(&model.Command{
		Trigger:          triggerGif,
		Description:      "Posts a Giphy GIF that matches the keyword(s)",
		DisplayName:      "Giphy command",
		AutoComplete:     true,
		AutoCompleteDesc: "Posts a Giphy GIF that matches the keyword(s)",
		AutoCompleteHint: "happy kitty",
	})
	if err != nil {
		return err
	}

	err = api.RegisterCommand(&model.Command{
		Trigger:          triggerGifs,
		Description:      "Shows a preview of 10 GIFS matching the keyword(s)",
		DisplayName:      "Giphy preview command",
		AutoComplete:     true,
		AutoCompleteDesc: "Shows a preview of 10 GIFS matching the keyword(s)",
		AutoCompleteHint: "happy kitty",
	})
	if err != nil {
		return err
	}

	return p.OnConfigurationChange()
}

func (p *GiphyPlugin) config() *GiphyPluginConfiguration {
	return p.configuration.Load().(*GiphyPluginConfiguration)
}

// OnConfigurationChange handles plugin configuration changes
func (p *GiphyPlugin) OnConfigurationChange() error {
	var configuration GiphyPluginConfiguration
	err := p.api.LoadPluginConfiguration(&configuration)
	p.configuration.Store(&configuration)
	return err
}

// OnDeactivate handles plugin deactivation
func (p *GiphyPlugin) OnDeactivate() error {
	p.enabled = false
	return nil
}

// ExecuteCommand returns a post that displays a GIF choosen using Giphy
func (p *GiphyPlugin) ExecuteCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.enabled {
		return nil, appError("Cannot execute command while the plugin is disabled.", nil)
	}
	if p.api == nil {
		return nil, appError("Cannot access the plugin API.", nil)
	}
	if strings.HasPrefix(args.Command, "/"+triggerGifs) {
		return p.executeCommandGifs(args.Command)
	}
	if strings.HasPrefix(args.Command, "/"+triggerGif) {
		return p.executeCommandGif(args.Command)
	}

	return nil, appError("Command trigger "+args.Command+"is not supported by this plugin.", nil)
}

// executeCommandGif returns a public post containing a matching GIF
func (p *GiphyPlugin) executeCommandGif(command string) (*model.CommandResponse, *model.AppError) {
	keywords := getCommandKeywords(command, triggerGif)
	config := p.config()
	gifURL, err := p.gifProvider.getGifURL(config, keywords)
	if err != nil {
		return nil, appError("Unable to get GIF URL", err)
	}

	text := applyResponseTemplate(config.ResponseTemplate, keywords, gifURL)
	return &model.CommandResponse{ResponseType: model.COMMAND_RESPONSE_TYPE_IN_CHANNEL, Text: text}, nil
}

// executeCommandGifs returns a private post containing a list of matching GIFs
func (p *GiphyPlugin) executeCommandGifs(command string) (*model.CommandResponse, *model.AppError) {
	keywords := getCommandKeywords(command, triggerGifs)
	gifURLs, err := p.gifProvider.getMultipleGifsURL(p.config(), keywords)
	if err != nil {
		return nil, appError("Unable to get GIF URL", err)
	}

	text := fmt.Sprintf(" *Suggestions for '%s':*", keywords)
	for i, url := range gifURLs {
		if i > 0 {
			text += "\t"
		}
		text += fmt.Sprintf("[![GIF for '%s'](%s)](%s)", keywords, url, url)
	}
	return &model.CommandResponse{ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL, Text: text}, nil
}

func getCommandKeywords(commandLine string, trigger string) string {
	return strings.Trim(strings.Replace(commandLine, "/"+trigger, "", 1), " ")
}

func applyResponseTemplate(template, keywords, gifURL string) string {
	r := strings.NewReplacer("##KEYWORDS##", keywords,
		"##GIF_URL##", gifURL,
		"##VIA_GIPHY##", "via ![giphy](https://giphy.com/static/img/favicon.png)")
	return r.Replace(template)
}

func appError(message string, err error) *model.AppError {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	return model.NewAppError("Giphy Plugin", message, nil, errorMessage, http.StatusBadRequest)
}

// Install the RCP plugin
func main() {
	plugin := GiphyPlugin{}
	plugin.gifProvider = &giphyProvider{}
	rpcplugin.Main(&plugin)
}
