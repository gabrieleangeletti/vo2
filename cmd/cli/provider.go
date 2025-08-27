package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gabrieleangeletti/stride/strava"
	"github.com/gabrieleangeletti/vo2/internal"
)

func NewProviderCmd(cfg Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Provider cli",
		Long:  `Provider cli`,
	}

	cmd.AddCommand(stravaCmd)
	stravaCmd.AddCommand(stravaWebhookCmd)
	stravaCmd.AddCommand(stravaAuthCmd())

	stravaWebhookCmd.AddCommand(stravaCreateWebhookCmd(cfg))
	stravaWebhookCmd.AddCommand(stravaGetWebhookSubscriptionsCmd())

	return cmd
}

var stravaCmd = &cobra.Command{
	Use:   "strava",
	Short: "Strava cli",
	Long:  `Strava cli`,
}

func stravaAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Generate Strava authorization URL",
		Long:  `Generate Strava authorization URL`,
		Run: func(cmd *cobra.Command, args []string) {
			baseURL := internal.GetSecret("API_BASE_URL", true)

			clientID := internal.GetSecret("STRAVA_CLIENT_ID", true)
			clientSecret := internal.GetSecret("STRAVA_CLIENT_SECRET", true)

			auth := strava.NewAuth(clientID, clientSecret)

			redirectURL := fmt.Sprintf("%s/providers/strava/auth/callback", baseURL)
			authURL := auth.GetAuthorizationUrl(redirectURL)

			fmt.Println("Please visit the following URL to authenticate:")
			fmt.Println(authURL)

			fmt.Print("Would you like to open this URL in your browser? (y/n): ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')

			if strings.TrimSpace(response) == "y" {
				err := internal.OpenURLInBrowser(authURL.String())
				if err != nil {
					log.Fatalf("Failed to open the browser: %w", err)
				}
			}

			os.Exit(0)
		},
	}
}

var stravaWebhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Strava webhook commands",
	Long:  `Strava webhook commands`,
}

func stravaCreateWebhookCmd(cfg Config) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create Strava webhook",
		Long:  `Create Strava webhook`,
		Run: func(cmd *cobra.Command, args []string) {
			clientID := internal.GetSecret("STRAVA_CLIENT_ID", true)
			clientSecret := internal.GetSecret("STRAVA_CLIENT_SECRET", true)
			callbackURL := internal.GetSecret("STRAVA_WEBHOOK_CALLBACK_URL", true)

			verification, err := internal.CreateWebhookVerification(cfg.DB)
			if err != nil {
				log.Fatal("Failed to create verification token:\n", err)
			}

			auth := strava.NewAuth(clientID, clientSecret)

			resp, err := auth.RegisterWebhook(callbackURL, verification.Token)
			if err != nil {
				err2 := internal.DeleteWebhookVerification(cfg.DB, verification)
				if err2 != nil {
					log.Fatal("Error deleting webhook verification:\n", err2)
				}

				log.Fatal("Error registering webhook:\n", err)
			}

			fmt.Printf("Webhook successfully registered, id: %d", resp.ID)
		},
	}
}

func stravaGetWebhookSubscriptionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-subscriptions",
		Short: "Get Strava webhook subscriptions",
		Long:  `Get Strava webhook subscriptions`,
		Run: func(cmd *cobra.Command, args []string) {
			clientID := internal.GetSecret("STRAVA_CLIENT_ID", true)
			clientSecret := internal.GetSecret("STRAVA_CLIENT_SECRET", true)

			auth := strava.NewAuth(clientID, clientSecret)

			subscriptions, err := auth.GetWebhookSubscriptions()
			if err != nil {
				log.Fatal("Error getting webhook subscriptions:\n", err)
			}

			fmt.Printf("Found %d subscriptions:\n", len(subscriptions))
			for _, sub := range subscriptions {
				fmt.Printf("%d: %s", sub.ID, sub.CallbackURL)
			}
		},
	}
}
