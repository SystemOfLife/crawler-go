# Simple recursive web crawler written in Golang

TODO:
1. Manageability: cancellation of goroutines using context
2. Ability to go deeper: adding a worker pool, semaphore for bypassing sites blocks, mb should add libs with headless Chrome for scrapping
3. Optimizations: Use of tokenizer instead of regexp for speed (but code would be less readable)
