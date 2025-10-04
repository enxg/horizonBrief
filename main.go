package main

import "C"
import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/ebitengine/oto/v3"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	_ "github.com/joho/godotenv/autoload"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
	"google.golang.org/genai"
)

type ConfigUser struct {
	Name string `json:"name"`
}

type ConfigLocation struct {
	Name                string  `json:"name"`
	FriendlyDescription string  `json:"friendly_description"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	Notes               string  `json:"notes"`
}

type ConfigCalendar struct {
	ID    string `json:"id"`
	Notes string `json:"notes"`
}

type ConfigGemini struct {
	Text  TextModel  `json:"text"`
	Voice VoiceModel `json:"voice"`
}

type TextModel struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type VoiceModel struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Voice  string `json:"voice"`
}

type Config struct {
	User      ConfigUser       `json:"user"`
	Locations []ConfigLocation `json:"locations"`
	Calendars []ConfigCalendar `json:"calendars"`
	Gemini    ConfigGemini     `json:"gemini"`
}

type User struct {
	Name     string `json:"name"`
	DateTime string `json:"dateTime"`
	Day      string `json:"day"`
}

type Location struct {
	Name                string         `json:"name"`
	FriendlyDescription string         `json:"friendly_description"`
	Notes               string         `json:"notes"`
	WeatherData         map[string]any `json:"weather_data"`
}

type Calendar struct {
	Name   string         `json:"name"`
	Notes  string         `json:"notes"`
	Events map[string]any `json:"events"`
}

type AIData struct {
	User      User       `json:"user"`
	Locations []Location `json:"locations"`
	Calendars []Calendar `json:"calendars"`
}

const weatherAPIUrl = "https://weather.googleapis.com/v1/forecast/hours:lookup?key=%s&location.latitude=%f&location.longitude=%f&hours=%d"

func main() {
	ctx := context.Background()

	app := fiber.New()
	cc := client.New()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	weatherAPIKey := os.Getenv("WEATHER_API_KEY")
	if weatherAPIKey == "" {
		panic("WEATHER_API_KEY environment variable is not set")
	}

	var config Config
	data, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	err = sonic.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	serviceCredentialsFile := "./service_account.json"
	cal, err := calendar.NewService(ctx, option.WithCredentialsFile(serviceCredentialsFile))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	gen, err := genai.NewClient(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	textConfig := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(config.Gemini.Text.Prompt, genai.RoleUser),
	}

	voiceConfig := &genai.GenerateContentConfig{
		ResponseModalities: []string{"AUDIO"},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: config.Gemini.Voice.Voice,
				},
			},
		},
	}

	app.Get("/day", func(c fiber.Ctx) error {
		ti := time.Now().In(time.Local)
		//ti := time.Unix(1759841121, 0).In(time.Local) // For testing purposes
		tis := ti.Format(time.RFC3339)

		aiData := AIData{
			User: User{
				Name:     config.User.Name,
				DateTime: tis,
				Day:      ti.Weekday().String(),
			},
		}

		for _, location := range config.Locations {
			res, err := cc.Get(fmt.Sprintf(weatherAPIUrl, weatherAPIKey, location.Latitude, location.Longitude, 24))
			if err != nil {
				return err
			}

			body := make(map[string]any)
			if err := res.JSON(&body); err != nil {
				return err
			}

			aiData.Locations = append(aiData.Locations, Location{
				Name:                location.Name,
				FriendlyDescription: location.FriendlyDescription,
				Notes:               location.Notes,
				WeatherData:         body,
			})
		}

		startOfDay := time.Date(ti.Year(), ti.Month(), ti.Day(), 0, 0, 0, 0, ti.Location())
		startOfTomorrow := startOfDay.Add(24 * time.Hour)
		timeMin := startOfDay.Format(time.RFC3339)
		timeMax := startOfTomorrow.Format(time.RFC3339)

		for _, c := range config.Calendars {
			ci, err := cal.Calendars.Get(c.ID).Do()
			if err != nil {
				log.Printf("Unable to retrieve calendar: %v", err)
				continue
			}

			events, err := cal.Events.
				List(c.ID).
				ShowDeleted(false).
				SingleEvents(true).
				TimeMin(timeMin).
				TimeMax(timeMax).
				OrderBy("startTime").
				Do()
			if err != nil {
				log.Printf("Unable to retrieve next ten of the user's events: %v", err)
				continue
			}

			aiData.Calendars = append(aiData.Calendars, Calendar{
				Name:  ci.Summary,
				Notes: c.Notes,
				Events: map[string]any{
					"items": events.Items,
				},
			})
		}

		err := c.SendString("Generating audio")
		if err != nil {
			return err
		}

		go func() {
			aiDataText, err := sonic.Marshal(aiData)
			if err != nil {
				log.Printf("Unable to marshal AI data: %v", err)
				return
			}

			text, err := gen.Models.GenerateContent(
				ctx,
				config.Gemini.Text.Model,
				genai.Text(string(aiDataText)),
				textConfig,
			)
			if err != nil {
				log.Printf("Unable to generate text: %v", err)
				return
			}

			voice, err := gen.Models.GenerateContent(
				ctx,
				config.Gemini.Voice.Model,
				genai.Text(config.Gemini.Voice.Prompt+text.Text()),
				voiceConfig,
			)
			if err != nil {
				log.Printf("Unable to generate voice: %v", err)
				return
			}

			meta := voice.Candidates[0].Content.Parts[0].InlineData

			op := &oto.NewContextOptions{}
			op.SampleRate = 24000
			op.ChannelCount = 1
			op.Format = oto.FormatSignedInt16LE

			audioReader := bytes.NewReader(meta.Data)

			println("Parsing audio...")

			otoCtx, readyChan, err := oto.NewContext(op)
			if err != nil {
				panic("oto.NewContext failed: " + err.Error())
			}

			<-readyChan

			println("Playing audio...")

			player := otoCtx.NewPlayer(audioReader)
			player.Play()

			for player.IsPlaying() {
				time.Sleep(time.Millisecond)
			}

			defer player.Close()
		}()

		return nil
	})

	app.Listen(":" + port)
}
