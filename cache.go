package cache

// A simple cache mechanism for testing performance. This is intended to be used for global state
// across requests, such as SiteConfig. The caller is responsible for re-adding values if the cache
// is missed.
// This is a very naive caching mechanism. It is only suitable for small caches at this point,
// as the expiry locks the set while checking

// Ideas to try here include:
//  * add to cache with a function that can be called in a goroutine to refresh the value on
//    expiry. set-and-forget. Duration still specified. This is actually important for permanently cycled
//	  caches like SiteTree, so that we can replace the value before invaliding the new one; otherwise multiple
//    requests will race to re-populate it, causing an unnecessary load spike.
//  * add to cache with a policy expiry function. The cache will poll the policy expiry functions.

import (
	"sync"
	"time"
)

// ValueGenerator is any function that when called generates a value. Used in perpetual cache entries.
type ValueGenerator func() interface{}

// Cache represents a cache instance. The cache will in general contain a number of elements. Each Cache
// operates independently, and can be used safely across multiple go-routines.
type Cache struct {
	// mutex to safely handle changes to the cache across goroutines.
	sync.Mutex

	// the map of entries.
	entries map[interface{}]*CacheEntry

	ticker *time.Ticker
	quit   chan bool
}

// CacheEntry represents the value of an entry in the cache. It primarily holds the current value, but also
// when the entry expires. CacheEntry instances can be perpetual or not. Perpetual cache entries have a
// ValueGenerator, a function that generates a refreshed value when called. Non-perpetual cache entries
// are deleted from the cache on expiry.
type CacheEntry struct {
	value  interface{}
	expiry time.Time

	// Indicates if this is a perpetual entry (true) or not (false). Perpetual entries must also
	// have fn and lifetime values.
	perpetual bool

	// for perpetual cache entries, this is the function used to refresh the value on expiry.
	fn ValueGenerator

	// for perpetual cache enties, this is the lifetime so we can keep re-generating.
	lifetime time.Duration
}

// NewCache returns a new, initialised Cache instance.
func NewCache() *Cache {
	c := &Cache{}
	c.entries = make(map[interface{}]*CacheEntry)

	c.startTimer()

	return c
}

// Free is required to cleanup before a cache is deleted. This ensures that the timer that invalidates
// cache entries is stopped.
func (c *Cache) Free() {
	c.quit <- true
}

// Store a key/value pair in the cache, with the specified lifetime. On expiry, the cache entry
// is just deleted from the cache.
func (c *Cache) Store(key interface{}, value interface{}, lifetime time.Duration) {
	entry := &CacheEntry{value: value, expiry: time.Now().Add(lifetime), perpetual: false}
	c.Lock()
	c.entries[key] = entry
	c.Unlock()
}

// Store a key/value pair in the cache, where the value comes from a function. On expiry, after
// the duration, the cache entry's value is recomputed by calling the function. The value is not
// replaced until the function has generated a new value, so multiple consumers of the cache
// entry will either get the old value or the new value, but will not each attempt to regenerate
// the entry. These cache entries can be deleted using Delete. Otherwise they remain for the duration
// of the cache and the program.
func (c *Cache) StorePerpetual(key interface{}, fn ValueGenerator, lifetime time.Duration) {
	entry := &CacheEntry{fn: fn, expiry: time.Now().Add(lifetime), lifetime: lifetime, perpetual: true}
	entry.value = fn()
	c.Lock()
	c.entries[key] = entry
	c.Unlock()
}

// Delete a cache entry by key. This can be used to eject a value before the lifetime duration,
// or delete a recurring entry such as those added with StorePerpetual
func (c *Cache) Delete(key interface{}) {
	delete(c.entries, key)
}

// Retrieve a value from the cache given it's key. Returns nil if there is no value.
func (c *Cache) Get(key interface{}) interface{} {
	c.Lock()
	entry := c.entries[key]
	c.Unlock()
	if entry == nil {
		return nil
	}
	return entry.value
}

// start up a 1 second ping to expiry cache entries past their expiry.
// @todo parameterise the cache ping, in milliseconds, with 1 second default.
func (c *Cache) startTimer() {
	c.ticker = time.NewTicker(time.Second)
	c.quit = make(chan bool)
	go func() {
		for {
			select {
			case <-c.ticker.C:
				n := time.Now().UnixNano()
				c.Lock()
				for k, v := range c.entries {
					e := v.expiry.UnixNano()
					if e <= n {
						c.expire(k, v)
					}
				}
				c.Unlock()
			case <-c.quit:
				c.ticker.Stop()
				return
			}
		}
	}()
}

// Handle expiry of a cache entry. If it is not perpetual, just remove it from the cache.
// If it is perpetual, execute the function to regenerate a new value.
func (c *Cache) expire(key interface{}, entry *CacheEntry) {
	if entry.perpetual {
		// entry is perpetual, so evaluate the function for a new value.
		nv := entry.fn()

		// Replace the value atomically
		c.Lock()

		// store the new value
		entry.value = nv

		// recompute the expiry
		entry.expiry = time.Now().Add(entry.lifetime)

		c.Unlock()
	} else {
		// not perpetual, just delete it.
		c.Delete(key)
	}
}
