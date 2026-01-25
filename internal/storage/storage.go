package storage

import (
	"encoding/json"
	"io/ioutil"
	"project/internal/models"
)

func ReadUsers() ([]models.User, error) {
	data, err := ioutil.ReadFile("data/users.json")
	if err != nil {
		return nil, err
	}

	var users []models.User
	err = json.Unmarshal(data, &users)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func WriteUsers(users []models.User) error {
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile("data/users.json", data, 0644)
}

func ReadArticles() ([]models.Article, error) {
	data, err := ioutil.ReadFile("data/articles.json")
	if err != nil {
		return nil, err
	}

	var articles []models.Article
	err = json.Unmarshal(data, &articles)
	if err != nil {
		return nil, err
	}

	return articles, nil
}

func WriteArticles(articles []models.Article) error {
	data, err := json.MarshalIndent(articles, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile("data/articles.json", data, 0644)
}
