package api

import (
	"net/http"
	"strconv"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func CreateArticle(c *gin.Context) {
	var article models.Article
	if err := c.ShouldBindJSON(&article); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	articles, err := storage.ReadArticles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read articles"})
		return
	}

	article.ID = len(articles) + 1
	articles = append(articles, article)

	if err := storage.WriteArticles(articles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write articles"})
		return
	}

	c.JSON(http.StatusCreated, article)
}

func GetArticle(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid article ID"})
		return
	}

	articles, err := storage.ReadArticles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read articles"})
		return
	}

	for _, article := range articles {
		if article.ID == id {
			c.JSON(http.StatusOK, article)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
}

func UpdateArticle(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid article ID"})
		return
	}

	var updatedArticle models.Article
	if err := c.ShouldBindJSON(&updatedArticle); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	articles, err := storage.ReadArticles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read articles"})
		return
	}

	for i, article := range articles {
		if article.ID == id {
			articles[i] = updatedArticle
			articles[i].ID = id

			if err := storage.WriteArticles(articles); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write articles"})
				return
			}

			c.JSON(http.StatusOK, articles[i])
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
}
