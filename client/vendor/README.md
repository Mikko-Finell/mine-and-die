# Vendor Libraries

Place third-party JavaScript modules in this directory so they can be imported by the client using standard ES module syntax:

```js
import { something } from "./vendor/some-library.js";
```

Keep filenames kebab-cased to avoid case-sensitive import issues across operating systems. Each library should export the functions you need so the rest of the client can consume them without referencing global variables.
