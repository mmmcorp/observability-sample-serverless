package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/go-playground/validator/v10"
)

type UpdateTodo struct {
	Status bool `json:"status"`
}

type CreateTodo struct {
	Task string `json:"task" validate:"required"`
}

var validate *validator.Validate = validator.New()

func router(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Received req %#v", req)

	httpMethod := req.HTTPMethod
	path := req.Path

	switch {
	case httpMethod == "GET" && path == "/api/task":
		return processGet(ctx, req)
	case httpMethod == "POST" && path == "/api/task":
		return processPost(ctx, req)
	case httpMethod == "PUT" && strings.HasPrefix(path, "/api/task/"):
		return processPut(ctx, req)
	case httpMethod == "PUT" && strings.HasPrefix(path, "/api/undoTask/"):
		return processPut(ctx, req)
	case httpMethod == "DELETE" && strings.HasPrefix(path, "/api/deleteTask/"):
		return processDelete(ctx, req)
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       "Not Found",
		}, nil
	}

}

func processGet(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id, ok := req.PathParameters["id"]
	if !ok {
		return processGetTodos(ctx)
	} else {
		return processGetTodo(ctx, id)
	}
}

func processGetTodo(ctx context.Context, id string) (events.APIGatewayProxyResponse, error) {
	log.Printf("Received GET todo request with id = %s", id)

	todo, err := getItem(ctx, id)
	if err != nil {
		return serverError(err)
	}

	if todo == nil {
		return clientError(http.StatusNotFound)
	}

	json, err := json.Marshal(todo)
	if err != nil {
		return serverError(err)
	}
	log.Printf("Successfully fetched todo item %s", json)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Origin":  "*",
		},
		Body: string(json),
	}, nil
}

func processGetTodos(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	log.Print("Received GET todos request")

	todos, err := listItems(ctx)
	if err != nil {
		return serverError(err)
	}

	json, err := json.Marshal(todos)
	if err != nil {
		return serverError(err)
	}
	log.Printf("Successfully fetched todos: %s", json)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Origin":  "*",
		},
		Body: string(json),
	}, nil
}

func processPost(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var createTodo CreateTodo
	err := json.Unmarshal([]byte(req.Body), &createTodo)
	if err != nil {
		log.Printf("Can't unmarshal body: %v", err)
		return clientError(http.StatusUnprocessableEntity)
	}

	err = validate.Struct(&createTodo)
	if err != nil {
		log.Printf("Invalid body: %v", err)
		return clientError(http.StatusBadRequest)
	}
	log.Printf("Received POST request with item: %+v", createTodo)

	res, err := insertItem(ctx, createTodo)
	if err != nil {
		return serverError(err)
	}
	log.Printf("Inserted new todo: %+v", res)

	json, err := json.Marshal(res)
	if err != nil {
		return serverError(err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       string(json),
		Headers: map[string]string{
			"Location":                     fmt.Sprintf("/todo/%s", res.Id),
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Origin":  "*",
		},
	}, nil
}

func processDelete(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id, ok := req.PathParameters["id"]
	if !ok {
		return clientError(http.StatusBadRequest)
	}
	log.Printf("Received DELETE request with id = %s", id)

	todo, err := deleteItem(ctx, id)
	if err != nil {
		return serverError(err)
	}

	if todo == nil {
		return clientError(http.StatusNotFound)
	}

	json, err := json.Marshal(todo)
	if err != nil {
		return serverError(err)
	}
	log.Printf("Successfully deleted todo item %+v", todo)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Origin":  "*",
		},
		Body: string(json),
	}, nil
}

func processPut(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id, ok := req.PathParameters["id"]
	if !ok {
		return clientError(http.StatusBadRequest)
	}

	var updateTodo UpdateTodo

	path := req.Path

	switch {
	case strings.HasPrefix(path, "/api/task/"):
		updateTodo = UpdateTodo{Status: true}
	case strings.HasPrefix(path, "/api/undoTask/"):
		log.Printf("here")
		updateTodo = UpdateTodo{Status: false}
	}

	res, err := updateItem(ctx, id, updateTodo)
	if err != nil {
		return serverError(err)
	}

	if res == nil {
		return clientError(http.StatusNotFound)
	}

	log.Printf("Updated todo: %+v", res)

	json, err := json.Marshal(res)
	if err != nil {
		return serverError(err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(json),
		Headers: map[string]string{
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Origin":  "*",
			"Location":                     fmt.Sprintf("/todo/%s", res.Id),
		},
	}, nil
}

func clientError(status int) (events.APIGatewayProxyResponse, error) {

	return events.APIGatewayProxyResponse{
		Body:       http.StatusText(status),
		StatusCode: status,
		Headers: map[string]string{
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Origin":  "*",
		},
	}, nil
}

func serverError(err error) (events.APIGatewayProxyResponse, error) {
	log.Println(err.Error())

	return events.APIGatewayProxyResponse{
		Body:       http.StatusText(http.StatusInternalServerError),
		StatusCode: http.StatusInternalServerError,
		Headers: map[string]string{
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Origin":  "*",
		},
	}, nil
}
