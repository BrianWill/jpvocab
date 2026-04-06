package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

type imageSources struct {
	UnsplashAvail bool
	PexelsAvail   bool
	PixabayAvail  bool
	BingAvail     bool
}

func checkImageSources() imageSources {
	return imageSources{
		UnsplashAvail: os.Getenv("UNSPLASH_ACCESS_KEY") != "",
		PexelsAvail:   os.Getenv("PEXELS_API_KEY") != "",
		PixabayAvail:  os.Getenv("PIXABAY_API_KEY") != "",
		BingAvail:     os.Getenv("BING_API_KEY") != "",
	}
}

func searchUnsplash(ctx context.Context, query string) (string, error) {
	key := os.Getenv("UNSPLASH_ACCESS_KEY")
	if key == "" {
		return "", errors.New("UNSPLASH_ACCESS_KEY not set")
	}
	endpoint := "https://api.unsplash.com/search/photos?per_page=1&query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Client-ID "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unsplash API: %s", resp.Status)
	}
	var result struct {
		Results []struct {
			URLs struct {
				Regular string `json:"regular"`
			} `json:"urls"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Unsplash response: %w", err)
	}
	if len(result.Results) == 0 || result.Results[0].URLs.Regular == "" {
		return "", errors.New("no Unsplash results")
	}
	return result.Results[0].URLs.Regular, nil
}

func searchPexels(ctx context.Context, query string) (string, error) {
	key := os.Getenv("PEXELS_API_KEY")
	if key == "" {
		return "", errors.New("PEXELS_API_KEY not set")
	}
	endpoint := "https://api.pexels.com/v1/search?per_page=1&query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Pexels API: %s", resp.Status)
	}
	var result struct {
		Photos []struct {
			Src struct {
				Medium string `json:"medium"`
			} `json:"src"`
		} `json:"photos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Pexels response: %w", err)
	}
	if len(result.Photos) == 0 || result.Photos[0].Src.Medium == "" {
		return "", errors.New("no Pexels results")
	}
	return result.Photos[0].Src.Medium, nil
}

func searchPixabay(ctx context.Context, query string) (string, error) {
	key := os.Getenv("PIXABAY_API_KEY")
	if key == "" {
		return "", errors.New("PIXABAY_API_KEY not set")
	}
	endpoint := "https://pixabay.com/api/?image_type=photo&safesearch=true&per_page=3&key=" +
		url.QueryEscape(key) + "&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Pixabay API: %s", resp.Status)
	}
	var result struct {
		Hits []struct {
			WebformatURL string `json:"webformatURL"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Pixabay response: %w", err)
	}
	if len(result.Hits) == 0 || result.Hits[0].WebformatURL == "" {
		return "", errors.New("no Pixabay results")
	}
	return result.Hits[0].WebformatURL, nil
}

func searchBing(ctx context.Context, query string) (string, error) {
	key := os.Getenv("BING_API_KEY")
	if key == "" {
		return "", errors.New("BING_API_KEY not set")
	}
	endpoint := "https://api.bing.microsoft.com/v7.0/images/search?count=1&safeSearch=Moderate&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Bing API: %s", resp.Status)
	}
	var result struct {
		Value []struct {
			ContentURL string `json:"contentUrl"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Bing response: %w", err)
	}
	if len(result.Value) == 0 || result.Value[0].ContentURL == "" {
		return "", errors.New("no Bing results")
	}
	return result.Value[0].ContentURL, nil
}
