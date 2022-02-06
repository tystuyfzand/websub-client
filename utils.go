package client

import (
    "crypto/sha1"
    "crypto/sha256"
    "crypto/sha512"
    "fmt"
    "hash"
    "meow.tf/websub/model"
    "net/url"
    "reflect"
    "strconv"
)

func remove(s []model.Subscription, i int) []model.Subscription {
    s[i] = s[len(s)-1]
    return s[:len(s)-1]
}

// newHash takes a string and returns a hash.Hash based on type.
func newHash(hasher string) func() hash.Hash {
    switch hasher {
    case "sha1":
        return sha1.New
    case "sha256":
        return sha256.New
    case "sha384":
        return sha512.New384
    case "sha512":
        return sha512.New
    }

    return nil
}

// encodeForm is a simple utility for encoding a struct to a form
func encodeForm(model interface{}) string {
    v := reflect.ValueOf(model)
    t := reflect.TypeOf(model)

    form := url.Values{}

    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)

        tag := field.Tag.Get(formTag)

        fieldValue := v.Field(i)

        switch field.Type.Kind() {
        case reflect.Bool:
            form.Set(tag, strconv.FormatBool(fieldValue.Bool()))
        case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
            form.Set(tag, strconv.FormatInt(fieldValue.Int(), 10))
        case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
            form.Set(tag, strconv.FormatUint(fieldValue.Uint(), 10))
        case reflect.Float32, reflect.Float64:
            form.Set(tag, fmt.Sprintf("%f", fieldValue.Float()))
        case reflect.String:
            form.Set(tag, fieldValue.String())
        }
    }

    return form.Encode()
}
