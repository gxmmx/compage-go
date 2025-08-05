package ctx

import "context"

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type key int

var appName key

func New(name string) context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, appName, name)
}

func GetAppName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if name, ok := ctx.Value(appName).(string); ok {
		return name
	}
	return ""
}
