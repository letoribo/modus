/*
 * Copyright 2024 Hypermode, Inc.
 */

package utils

import (
	"context"
	"log"
	"runtime"
	"strings"
	"time"

	"hmruntime/config"

	"github.com/getsentry/sentry-go"
)

// The Sentry DSN for the Hypermode Runtime project (not a secret)
const sentryDsn = "https://d0de28f77651b88c22af84598882d60a@o4507057700470784.ingest.us.sentry.io/4507153498636288"

var rootSourcePath string

func InitSentry(rootPath string) {
	rootSourcePath = rootPath
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         sentryDsn,
		Environment: config.GetEnvironmentName(),
		Release:     config.GetVersionNumber(),
		BeforeSend:  sentryBeforeSend,

		// Note - We use Prometheus for _metrics_ (see hmruntime/metrics package).
		// However, we can still use Sentry for _tracing_ to allow us to improve performance of the Runtime.
		// We should only trace code that we expect to run in a roughly consistent amount of time.
		// For example, we should not trace a user's function execution, or the outer GraphQL request handling,
		// because these can take an arbitrary amount of time depending on what the user's code does.
		// Instead, we should trace the runtime's startup, storage, secrets retrieval, schema generation, etc.
		// That way we can trace performance issues in the runtime itself, and let Sentry correlate them with
		// any errors that may have occurred.
		EnableTracing:    true,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		// We don't have our logger yet, so just log to stderr.
		log.Fatalf("sentry.Init: %s", err)
	}
}

func FlushSentryEvents() {
	sentry.Flush(5 * time.Second)
}

func NewSentryTransactionForCurrentFunc(ctx context.Context) (*sentry.Span, context.Context) {
	transaction := sentry.StartTransaction(ctx, GetFuncName(2), sentry.WithOpName("function"))
	return transaction, transaction.Context()
}

func NewSentrySpanForCurrentFunc(ctx context.Context) *sentry.Span {
	span := sentry.StartSpan(ctx, "function")
	span.Description = GetFuncName(2)
	return span
}

func GetFuncName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "?"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "?"
	}

	name := fn.Name()
	return TrimStringBefore(name, "/")
}

func sentryBeforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {

	// Exclude user-visible errors from being reported to Sentry, because they are
	// caused by user code, and thus are not actionable by us.
	if event.Extra["user_visible"] == "true" {
		return nil
	}

	// Adjust the stack trace to show relative paths to the source code.
	for _, e := range event.Exception {
		st := *e.Stacktrace
		for i, f := range st.Frames {
			if f.Filename == "" && f.AbsPath != "" && strings.HasPrefix(f.AbsPath, rootSourcePath) {
				f.Filename = f.AbsPath[len(rootSourcePath):]
				st.Frames[i] = f
			}
		}
	}

	return event
}