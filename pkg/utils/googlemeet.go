package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GenerateMeetLink — avtomatik OAuth flow bajaradi va yangi Google Meet link qaytaradi.
func GenerateMeetLink() string {
	b, err := os.ReadFile("./configs/credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
		return ""
	}

	// redirect URI — credentials.json bilan mos bo‘lishi kerak
	config.RedirectURL = "http://localhost:8085"

	client := getClientAuto(config)

	link, err := generate(client)
	if err != nil {
		log.Fatalf("Unable to generate Meet link: %v", err)
		return ""
	}
	return link
}

// getClientAuto — avtomatik token olish (localhost redirect orqali)
func getClientAuto(config *oauth2.Config) *http.Client {
	tokFile := "./configs/token.json"

	// Avval token.json mavjudligini tekshiramiz
	tok, err := tokenFromFile(tokFile)
	if err == nil {
		// access token avtomatik yangilanadi, agar refresh_token mavjud bo‘lsa
		return config.Client(context.Background(), tok)
	}

	// token yo‘q bo‘lsa — OAuth flow boshlaymiz
	port := "8085"
	codeCh := make(chan string)
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code not found", http.StatusBadRequest)
			return
		}

		fmt.Fprint(w, "✅ Authorization successful! You can close this window.")
		codeCh <- code

		go func() {
			time.Sleep(1 * time.Second)
			_ = srv.Shutdown(context.Background())
		}()
	})

	// Avtorizatsiya havolasini chiqaramiz
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Println("🌐 Open this link in your browser:\n", authURL)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server stopped: %v", err)
		}
	}()

	// Redirectdan kelgan code’ni kutamiz
	code := <-codeCh
	fmt.Println("✅ Authorization code received")

	// code orqali token olish
	tok, err = config.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("Unable to exchange code for token: %v", err)
	}

	saveToken(tokFile, tok)
	fmt.Println("💾 Token saved successfully")

	return config.Client(context.Background(), tok)
}

// tokenFromFile — mavjud tokenni o‘qiydi
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken — tokenni faylga saqlaydi
func saveToken(path string, token *oauth2.Token) {
	if err := os.MkdirAll("./configs", 0755); err != nil {
		log.Fatalf("Unable to create config folder: %v", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to save oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// generate — Google Meet event yaratadi va Meet link qaytaradi
func generate(client *http.Client) (string, error) {
	ctx := context.Background()
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", err
	}

	event := &calendar.Event{
		Summary:     "Auto Generated Meeting",
		Description: "Meeting generated automatically by Go",
		Start: &calendar.EventDateTime{
			DateTime: "2025-10-10T10:00:00+05:00",
			TimeZone: "Asia/Tashkent",
		},
		End: &calendar.EventDateTime{
			DateTime: "2025-10-10T10:30:00+05:00",
			TimeZone: "Asia/Tashkent",
		},
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: fmt.Sprintf("req-%d", time.Now().Unix()),
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		},
	}

	res, err := srv.Events.Insert("primary", event).ConferenceDataVersion(1).Do()
	if err != nil {
		return "", fmt.Errorf("event insert failed: %w", err)
	}

	if res.HangoutLink != "" {
		return res.HangoutLink, nil
	}

	return "", fmt.Errorf("no HangoutLink in the response")
}
