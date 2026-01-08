package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	fmt.Println("Connecting to Elasticsearch at http://localhost:9200...")
	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		panic(fmt.Sprintf("Error creating client: %s", err))
	}
	res, err := esClient.Info()
	if err != nil {
		panic(fmt.Sprintf("Error getting response: %s", err))
	}
	defer res.Body.Close()
	fmt.Println("Connected to Elasticsearch")

	// Setup Dependnecies
	esWrapper := &database.ElasticsearchClient{Client: esClient}
	tmplRepo := database.NewTemplateRepository(esWrapper, logger)
	tmplService := usecases.NewTemplateService(tmplRepo, logger)

	ctx := context.Background()
	appA := "App_A_" + time.Now().Format("20060102150405")
	appB := "App_B_" + time.Now().Format("20060102150405")

	// 1. Create Template for App A
	req := &template.CreateRequest{
		AppID:     appA,
		Name:      "Secret Template",
		Body:      "Should not be seen by App B",
		Channel:   "email",
		Locale:    "en-US",
		CreatedBy: "tester",
	}
	tmpl, err := tmplService.Create(ctx, req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created Template %s for %s\n", tmpl.ID, appA)

	// 2. Try to Get with App B
	_, err = tmplService.GetByID(ctx, tmpl.ID, appB)
	if err == nil {
		fmt.Printf("FAIL: App B was able to access App A's template!\n")
		os.Exit(1)
	}
	if err.Error() == "template not found" {
		fmt.Printf("SUCCESS: App B blocked from accessing App A's template (GetByID)\n")
	} else {
		fmt.Printf("Unexpected error: %v\n", err)
	}

	// 3. Try to Update with App B
	updateBody := "Hacked by B"
	updateReq := &template.UpdateRequest{Body: &updateBody, UpdatedBy: "hacker"}
	_, err = tmplService.Update(ctx, tmpl.ID, appB, updateReq)
	if err == nil {
		fmt.Printf("FAIL: App B was able to update App A's template!\n")
		os.Exit(1)
	}
	if err.Error() == "template not found" {
		fmt.Printf("SUCCESS: App B blocked from updating App A's template\n")
	} else {
		fmt.Printf("Unexpected error: %v\n", err)
	}

	// 4. Try to Delete with App B
	err = tmplService.Delete(ctx, tmpl.ID, appB)
	if err == nil {
		fmt.Printf("FAIL: App B was able to delete App A's template!\n")
		os.Exit(1)
	}
	if err.Error() == "template not found" { // Service returns "template not found" on mismatch
		fmt.Printf("SUCCESS: App B blocked from deleting App A's template\n")
	} else {
		fmt.Printf("Unexpected error: %v\n", err)
	}

	// 5. Verify App A can access
	got, err := tmplService.GetByID(ctx, tmpl.ID, appA)
	if err != nil {
		fmt.Printf("FAIL: App A cannot access its own template: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("SUCCESS: App A verified access to template %s\n", got.ID)

	// Cleanup
	tmplService.Delete(ctx, tmpl.ID, appA)
}
