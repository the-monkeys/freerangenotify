package docgen

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/container"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/routes"
	"go.uber.org/zap"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

// RouteInventory registers the real router without opening sockets or constructing
// external clients. No request is executed with these synthetic dependencies.
func RouteInventory(enabled bool) (result []string) {
	for key := range RouteHandlers(enabled) {
		result = append(result, key)
	}
	sort.Strings(result)
	return
}

func NewInventoryRouter(enabled bool) *fiber.App {
	c := &container.Container{}
	value := reflect.ValueOf(c).Elem()
	for i := 0; i < value.NumField(); i++ {
		f := value.Field(i)
		if f.CanSet() && f.Kind() == reflect.Pointer {
			f.Set(reflect.New(f.Type().Elem()))
		}
	}
	c.Logger = zap.NewNop()
	c.Config.Licensing.DeploymentMode = "hosted"
	c.Config.Security.OpsEnabled = enabled
	if !enabled {
		for i := 0; i < value.NumField(); i++ {
			field := value.Type().Field(i)
			f := value.Field(i)
			if f.CanSet() && f.Kind() == reflect.Pointer && strings.HasSuffix(field.Name, "Handler") {
				f.Set(reflect.Zero(f.Type()))
			}
		}
		// Unconditionally mounted handlers must remain present.
		for _, name := range []string{"AuthHandler", "HealthHandler", "SSEHandler", "PlaygroundHandler", "UserHandler", "ApplicationHandler", "NotificationHandler", "TemplateHandler", "PresenceHandler", "LicensingHandler", "QuickSendHandler", "BillingHandler", "CustomProviderHandler", "AdminHandler", "AnalyticsHandler"} {
			f := value.FieldByName(name)
			f.Set(reflect.New(f.Type().Elem()))
		}
	}
	app := fiber.New()
	routes.SetupRoutes(app, c)
	return app
}
func RouteHandlers(enabled bool) map[string]string {
	app := NewInventoryRouter(enabled)
	defer app.Shutdown()
	seen := map[string]string{}
	for _, r := range app.GetRoutes(true) {
		if r.Method == "HEAD" || r.Method == "OPTIONS" {
			continue
		}
		path := strings.TrimRight(r.Path, "/")
		if path == "" {
			path = "/"
		}
		parts := strings.Split(path, "/")
		for i, p := range parts {
			if strings.HasPrefix(p, ":") {
				parts[i] = "{" + strings.TrimPrefix(p, ":") + "}"
			}
		}
		seen[fmt.Sprintf("%s %s", r.Method, strings.Join(parts, "/"))] = runtime.FuncForPC(reflect.ValueOf(r.Handlers[len(r.Handlers)-1]).Pointer()).Name()
	}
	return seen
}

// AllRouteInventory includes legacy routes disabled by the ops-plane cutover.
func AllRouteInventory() []string {
	seen := map[string]bool{}
	for _, enabled := range []bool{true, false} {
		for _, key := range RouteInventory(enabled) {
			seen[key] = true
		}
	}
	var result []string
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
