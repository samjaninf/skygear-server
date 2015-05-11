package pq

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/oursky/ourd/oddb"
)

func isDeviceNotFound(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23503" && pqErr.Constraint == "_subscription_device_id_fkey"
	}

	return false
}

type notificationInfoValue oddb.NotificationInfo

func (info notificationInfoValue) Value() (driver.Value, error) {
	return json.Marshal(info)
}

func (info *notificationInfoValue) Scan(value interface{}) error {
	if value == nil {
		*info = notificationInfoValue{}
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		fmt.Errorf("oddb: unsupported Scan pair: %T -> %T", value, info)
	}

	return json.Unmarshal(b, info)
}

type queryValue oddb.Query

func (query queryValue) Value() (driver.Value, error) {
	return json.Marshal(query)
}

func (query *queryValue) Scan(value interface{}) error {
	if value == nil {
		*query = queryValue{}
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		fmt.Errorf("oddb: unsupported Scan pair: %T -> %T", value, query)
	}

	return json.Unmarshal(b, query)
}

func (db *database) GetSubscription(key string, subscription *oddb.Subscription) error {
	err := psql.Select("device_id", "type", "notification_info", "query").
		From(db.tableName("_subscription")).
		Where("id = ? and user_id = ?", key, db.userID).
		RunWith(db.Db.DB).
		QueryRow().
		Scan(
		&subscription.DeviceID,
		&subscription.Type,
		(*notificationInfoValue)(&subscription.NotificationInfo),
		(*queryValue)(&subscription.Query))

	if err == sql.ErrNoRows {
		return oddb.ErrSubscriptionNotFound
	} else if err != nil {
		return err
	}

	subscription.ID = key

	return nil
}

func (db *database) SaveSubscription(subscription *oddb.Subscription) error {
	if subscription.ID == "" {
		return errors.New("empty id")
	}
	if subscription.Type == "" {
		return errors.New("empty type")
	}
	if subscription.Query.Type == "" {
		return errors.New("empty query type")
	}
	if subscription.DeviceID == "" {
		return errors.New("empty device id")
	}

	pkData := map[string]interface{}{
		"id":      subscription.ID,
		"user_id": db.userID,
	}

	data := map[string]interface{}{
		"device_id":         subscription.DeviceID,
		"type":              subscription.Type,
		"notification_info": notificationInfoValue(subscription.NotificationInfo),
		"query":             queryValue(subscription.Query),
	}

	sql, args := upsertQuery(db.tableName("_subscription"), pkData, data)
	_, err := db.Db.Exec(sql, args...)

	if isDeviceNotFound(err) {
		return oddb.ErrDeviceNotFound
	}

	return err
}

func (db *database) DeleteSubscription(key string) error {
	result, err := psql.Delete(db.tableName("_subscription")).
		Where("id = ? AND user_id = ?", key, db.userID).
		RunWith(db.Db.DB).
		Exec()

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return oddb.ErrSubscriptionNotFound
	} else if rowsAffected > 1 {
		panic(fmt.Errorf("want 1 rows updated, got %v", rowsAffected))
	}

	return nil
}

func (db *database) GetMatchingSubscription(record *oddb.Record) []oddb.Subscription { return nil }