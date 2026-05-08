package redis
// func TestCacheLock_StampedeProtection(t *testing.T) {
// 	// نیاز به Redis واقعی (می‌تونیم testcontainer یا mock Redis استفاده کنیم)
// 	// ...
// 	callCount := 0
// 	fetchFn := func() (interface{}, error) {
// 		callCount++
// 		// شبیه‌سازی یک کار سنگین
// 		time.Sleep(100 * time.Millisecond)
// 		return &entity.ModerationResult{ID: "result", ContentID: "c1", IsApproved: true}, nil
// 	}

// 	var wg sync.WaitGroup
// 	for i := 0; i < 100; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			_, err := redis.WithCacheLock(context.Background(), store, "key", 10*time.Second, 5*time.Second, 5, 20*time.Millisecond, fetchFn)
// 			assert.NoError(t, err)
// 		}()
// 	}
// 	wg.Wait()
// 	assert.Equal(t, 1, callCount, "fetchFn must be called exactly once")
// }