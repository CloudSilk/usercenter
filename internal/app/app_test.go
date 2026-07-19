package app

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestAPPPropsForeignKeyMatchesAPPPrimaryKey(t *testing.T) {
	appSchema, err := schema.Parse(&APP{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	relation := appSchema.Relationships.Relations["Props"]
	require.NotNil(t, relation)
	require.Len(t, relation.References, 1)

	reference := relation.References[0]
	require.Equal(t, "id", reference.PrimaryKey.DBName)
	require.Equal(t, "app_id", reference.ForeignKey.DBName)
	require.Equal(t, reference.PrimaryKey.DataType, reference.ForeignKey.DataType)
	require.Equal(t, reference.PrimaryKey.Size, reference.ForeignKey.Size)
}
