# cache

## Introduction

'cache' is a simple golang library for handling caching of things.

The main feature provided is a Cache type, which represents a cache that is a map of keys to values (both interface{}). Values are programmatically added to the cache, and can have:

 *  An expiry time, when they are removed from the cache.
 *  A value generator function that will be called automatically when the
    cache entry expires, thus making the cached value perpetual but updated
    automatically at a nominated frequency.

Cache objects are safe to share between goroutines.

## Import

Use:

    go get github.com/mrmorphic/cache

Then in your program:

    import (
        "github.com/mrmorphic/cache"
    )

## Usage

The following is a simple program fragment that shows the basic usage of the library:

    import (
        "github.com/mrmorphic/cache"
        "time"
    )

    func myfunc() {
        // create a Cache instance
        cache := NewCache()

        // Store a value to the cache, to expire in 30 seconds
        cache.Store("fred", "fish", time.Second*30)

        v := cache.Get("somekey")
    }

The Store method creates a cached entry with a simple expiry.

The following example shows the use of a perpetual cache entry, which will remain in the cache but get automatically refreshed on expiry.

    cache.StorePerpetual("mykey", func() int {
        ... calculate someValue
        return someValue
    }, time.Second*30) {

When the expiry time has been reached, the function is first called to generate a new value, and the cache entry's value is replaced. This means that consumers of the cache don't get a cache miss while the new value is generated. It does mean that the old value will continue to be returned until the generator function has completed.

To delete any cache entry, including perpetual cache entries, use Delete.

    cache.Delete("somekey")
