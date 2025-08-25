package main

import (
	"bufio"
	"fmt"
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

	return cmd
}

var stravaCmd = &cobra.Command{
	Use:   "strava",
	Short: "Strava cli customised for QPeaks",
	Long:  `Strava cli customised for QPeaks.`,
}

func NewStravaImportActivitySummaryCmd(cfg Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Generate Strava authorization URL",
		Long:  `Generate Strava authorization URL`,
		Run: func(cmd *cobra.Command, args []string) {
			baseURL := internal.GetSecret("API_BASE_URL", true)

			clientID := internal.GetSecret("STRAVA_CLIENT_ID", true)
			clientSecret := internal.GetSecret("STRAVA_CLIENT_SECRET", true)

			auth := strava.NewAuth(clientID, clientSecret)

			redirectURL := fmt.Sprintf("%s/providers/auth/strava/callback", baseURL)
			authURL := auth.GetAuthorizationUrl(redirectURL)

			fmt.Println("Please visit the following URL to authenticate:")
			fmt.Println(authURL)

			fmt.Print("Would you like to open this URL in your browser? (y/n): ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')

			if strings.TrimSpace(response) == "y" {
				err := internal.OpenURLInBrowser(authURL.String())
				if err != nil {
					fmt.Println("Failed to open the browser:", err)
				}
			}

			os.Exit(0)
		},
	}

	return cmd
}
