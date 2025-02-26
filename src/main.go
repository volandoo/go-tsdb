package main

import (
	"log"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the server",
		Long:  `Start the server with specific arguments`,
	}

	rootCmd.Flags().StringP("secret-key", "s", "", "The secret key for the server")
	rootCmd.Flags().StringArrayP("collection", "c", []string{}, "The collection names followed by colon and ttl in minutes. Accepts wildcards. Example: -c 'public:60' -c 'group.*:120'")
	rootCmd.Flags().StringP("storage-dir", "d", "", "The directory to store the data, if not set, data will not be stored on disk")
	rootCmd.Flags().IntP("storage-interval", "i", 0, "The interval to flush the data to the storage in seconds, if not set, data will not be flushed to the storage")

	rootCmd.Execute()

	secretKey, err := rootCmd.Flags().GetString("secret-key")
	if err != nil {
		log.Fatal(err)
	}
	collections, err := rootCmd.Flags().GetStringArray("collection")
	if err != nil {
		log.Fatal(err)
	}
	storageDir, err := rootCmd.Flags().GetString("storage-dir")
	if err != nil {
		log.Fatal(err)
	}
	storageInterval, err := rootCmd.Flags().GetInt("storage-interval")
	if err != nil {
		log.Fatal(err)
	}

	if secretKey == "" {
		log.Fatal("secret-key is not set")
	}

	if len(collections) == 0 {
		log.Fatal("collection is not set")
	}

	// check that collections items have a ttl
	for _, collection := range collections {
		parts := strings.Split(collection, ":")
		if len(parts) != 2 {
			log.Fatal("collection must have a ttl", collection)
		}
	}
	log.Printf("secret-key: %s", secretKey)
	log.Printf("collections: %v", collections)
	log.Printf("storage-dir: %s", storageDir)
	log.Printf("storage-interval: %d", storageInterval)

	if err := startServer(secretKey, collections, storageDir, storageInterval); err != nil {
		log.Fatal(err)
	}
}
