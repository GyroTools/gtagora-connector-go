package utils

import "net/url"

func ValidateURL(urlString string) (string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}

	if u.Scheme == "" {
		u.Scheme = "https"
		u, err = url.Parse(u.String())
		if err != nil {
			return "", err
		}
	}

	// Remove the path
	u.Path = ""

	// Get the modified URL without the path
	cleanURL := u.String()
	return cleanURL, nil
}
