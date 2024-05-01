/*
 * Copyright 2024 Hypermode, Inc.
 */

package engine

import (
	"fmt"
	"sync"

	"context"
	"strings"

	"hmruntime/config"
	"hmruntime/graphql/datasource"
	"hmruntime/graphql/schemagen"
	"hmruntime/logger"
	"hmruntime/plugins"
	"hmruntime/utils"

	"github.com/wundergraph/graphql-go-tools/execution/engine"
	gql "github.com/wundergraph/graphql-go-tools/execution/graphql"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/engine/plan"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/engine/resolve"
)

var instance *engine.ExecutionEngine
var mutex sync.RWMutex

// GetEngine provides thread-safe access to the current GraphQL execution engine.
func GetEngine() *engine.ExecutionEngine {
	mutex.RLock()
	defer mutex.RUnlock()
	return instance
}

func setEngine(engine *engine.ExecutionEngine) {
	mutex.Lock()
	defer mutex.Unlock()
	instance = engine
}

func Activate(ctx context.Context, metadata plugins.PluginMetadata) error {
	span := utils.NewSentrySpanForCurrentFunc(ctx)
	defer span.Finish()

	schema, err := generateSchema(ctx, metadata)
	if err != nil {
		return err
	}

	datasourceConfig, err := getDatasourceConfig(ctx, schema)
	if err != nil {
		return err
	}

	engine, err := makeEngine(ctx, schema, datasourceConfig)
	if err != nil {
		return err
	}

	setEngine(engine)
	return nil
}

func generateSchema(ctx context.Context, metadata plugins.PluginMetadata) (*gql.Schema, error) {
	span := utils.NewSentrySpanForCurrentFunc(ctx)
	defer span.Finish()

	schemaContent, err := schemagen.GetGraphQLSchema(ctx, metadata, true)
	if err != nil {
		return nil, err
	}

	if utils.HypermodeDebugEnabled() {
		if config.UseJsonLogging {
			logger.Debug(ctx).Str("schema", schemaContent).Msg("Generated schema")
		} else {
			fmt.Printf("\n%s\n", schemaContent)
		}
	}

	schema, err := gql.NewSchemaFromString(schemaContent)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

func getDatasourceConfig(ctx context.Context, schema *gql.Schema) (plan.DataSourceConfiguration[datasource.Configuration], error) {
	span := utils.NewSentrySpanForCurrentFunc(ctx)
	defer span.Finish()

	queryTypeName := schema.QueryTypeName()
	queryFieldNames := getAllQueryFields(ctx, schema)
	rootNodes := []plan.TypeField{
		{
			TypeName:   queryTypeName,
			FieldNames: queryFieldNames,
		},
	}

	var childNodes []plan.TypeField
	for _, f := range queryFieldNames {
		fields := schema.GetAllNestedFieldChildrenFromTypeField(queryTypeName, f, gql.NewSkipReservedNamesFunc())
		for _, field := range fields {
			childNodes = append(childNodes, plan.TypeField{
				TypeName:   field.TypeName,
				FieldNames: field.FieldNames,
			})
		}
	}

	return plan.NewDataSourceConfiguration(
		datasource.DataSourceName,
		&datasource.Factory[datasource.Configuration]{Ctx: ctx},
		&plan.DataSourceMetadata{RootNodes: rootNodes, ChildNodes: childNodes},
		datasource.Configuration{},
	)
}

func makeEngine(ctx context.Context, schema *gql.Schema, datasourceConfig plan.DataSourceConfiguration[datasource.Configuration]) (*engine.ExecutionEngine, error) {
	span := utils.NewSentrySpanForCurrentFunc(ctx)
	defer span.Finish()

	engineConfig := engine.NewConfiguration(schema)
	engineConfig.SetDataSources([]plan.DataSource{datasourceConfig})

	resolverOptions := resolve.ResolverOptions{
		MaxConcurrency:               1024,
		PropagateSubgraphErrors:      true,
		SubgraphErrorPropagationMode: resolve.SubgraphErrorPropagationModePassThrough,
	}

	adapter := newLoggerAdapter(ctx)
	return engine.NewExecutionEngine(ctx, adapter, engineConfig, resolverOptions)
}

func getAllQueryFields(ctx context.Context, s *gql.Schema) []string {
	span := utils.NewSentrySpanForCurrentFunc(ctx)
	defer span.Finish()

	doc := s.Document()
	queryTypeName := s.QueryTypeName()

	fields := make([]string, 0)
	for _, objectType := range doc.ObjectTypeDefinitions {
		typeName := doc.Input.ByteSliceString(objectType.Name)
		if typeName == queryTypeName {
			for _, fieldRef := range objectType.FieldsDefinition.Refs {
				field := doc.FieldDefinitions[fieldRef]
				fieldName := doc.Input.ByteSliceString(field.Name)
				if !strings.HasPrefix(fieldName, "__") {
					fields = append(fields, fieldName)
				}
			}
			break
		}
	}

	return fields
}