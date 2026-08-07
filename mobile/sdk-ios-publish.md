# Hinkal iOS SDK Publishing

The following are already configured:

- GitHub repository
- `ios-release` branch
- Swift Package Manager (`Package.swift`)
- GitHub Release workflow

No additional signing or publishing credentials are required.

---

## Release Process

### 1. Generate the iOS SDK

From the repository root:

```bash
cd libs/go
make gomobile-bind-ios
```

This generates:

```
dist/go/ios/
└── Hinkal.xcframework
```

---

### 2. Create the XCFramework archive

```bash
cd dist/go/ios
zip -r Hinkal.xcframework.zip Hinkal.xcframework
```

Verify the archive:

```bash
unzip -l Hinkal.xcframework.zip
```

---

### 3. Compute the checksum

```bash
swift package compute-checksum Hinkal.xcframework.zip
```

Copy the generated SHA-256 checksum.

---

### 4. Update `Package.swift`

Located at the repository root.

Update:

- Release version in the URL
- SHA-256 checksum

Example:

```swift
.binaryTarget(
    name: "Hinkal",
    url: "https://github.com/Hinkal-Protocol/Hinkal-Protocol/releases/download/0.1.1/Hinkal.xcframework.zip",
    checksum: "<NEW_CHECKSUM>"
)
```

---

### 5. Commit the changes

```bash
git checkout ios-release

git add Package.swift
git commit -m "Release iOS SDK <VERSION>"
git push origin ios-release
```

---

### 6. Create a release tag

```bash
git tag <VERSION>
git push origin <VERSION>
```

Example:

```bash
git tag 0.1.1
git push origin 0.1.1
```

The tag **must** point to the commit containing the updated `Package.swift`.

---

### 7. Create GitHub Release

Open:

```
GitHub → Releases → Draft new release
```

Use:

```
Tag:
<VERSION>

Title:
Hinkal iOS SDK <VERSION>
```

Upload:

```
dist/go/ios/Hinkal.xcframework.zip
```

Publish the release.

After publishing, the release assets should contain:

```
Hinkal.xcframework.zip
Source code (zip)
Source code (tar.gz)
```

---

### 8. Verify installation

Open Xcode.

```
File
→ Add Package Dependencies
```

Repository:

```
https://github.com/Hinkal-Protocol/Hinkal-Protocol
```

Select the desired version and import:

```swift
import Hinkal
```

If the package resolves successfully, the release is complete.

---
