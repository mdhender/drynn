// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func LoadDotfiles(env string) error {
	switch env {
	case "development":
	case "test":
	case "production":
	case "":
		return fmt.Errorf("missing environment")
	default:
		return fmt.Errorf("unknown environment")
	}
	//
	for _, path := range []string{
		".env." + env + ".local", // 1st priority: .env.{{ENV}}.local
		".env.local",             // 2nd priority: .env.local (except in test)
		".env." + env,            // 3rd priority: .env.{{ENV}}
		".env",                   // 4th priority: .env
	} {
		if env == "test" && path == ".env.local" {
			// .env.local is excluded in test for historical reasons.
			// if you're bored, read https://github.com/bkeepers/dotenv/issues/418
			continue
		}
		if !isfile(path) {
			// env file doesn't exist or isn't a file, so skip it
			continue
		}
		err := godotenv.Load(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// isfile returns true only if the path exists and is a regular file
func isfile(path string) bool {
	sb, err := os.Stat(path)
	return err == nil && sb.Mode().IsRegular()
}
