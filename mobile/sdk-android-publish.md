# Hinkal Android SDK Publishing

The following have already been configured:

- Maven Central namespace: `io.hinkal`
- Group ID: `io.hinkal`
- Artifact ID: `hinkal-android`
- GPG signing key
- Maven Central publishing token
- `publish-central.sh`
- Login using Hinkal team account

## Release Process

### 1. Generate Android bindings

From the repository root:

```bash
cd libs/go
make gomobile-bind
```

This generates:

```
dist/go/android/
├── hinkal.aar
└── hinkal-sources.jar
```

---

### 2. Update SDK version

Edit:

```
gradle.properties
```

Update:

```properties
sdkVersion=x.y.z (e.x. 0.1.0 -> 0.1.1)
```

---

### 3. Export publishing credentials

Replace the placeholders with the stored publishing credentials.

```bash
export MAVEN_CENTRAL_USERNAME='tJDwsR'
export MAVEN_CENTRAL_PASSWORD='3FLQwtXgQash8KI4T3OFu97uWczYsZOXC'

export GPG_SIGNING_KEY="$(
gpg --armor --export-secret-keys E234CEC494E8979420D1259AD4E53FEFD10DDBBA
)"

export GPG_SIGNING_PASSWORD='Hinkal123!'
```

Verify:

```bash
test -n "$MAVEN_CENTRAL_USERNAME" && echo "Central username configured"
test -n "$MAVEN_CENTRAL_PASSWORD" && echo "Central password configured"
test -n "$GPG_SIGNING_KEY" && echo "GPG key configured"
test -n "$GPG_SIGNING_PASSWORD" && echo "GPG password configured"
```

---

### 4. Verify publication

```bash
gradle clean publishToMavenLocal
```

Expected:

```
BUILD SUCCESSFUL
```

---

### 5. Create the signed Maven bundle

```bash
gradle clean publishReleasePublicationToLocalStagingRepository
```

Artifacts are created under:

```
build/staging-deploy/
```

---

### 6. Upload to Maven Central

```bash
chmod +x publish-central.sh
./publish-central.sh
```

Example output:

```
==> Building Central bundle...
==> Uploading to Central Portal (USER_MANAGED)...
Deployment ID:
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

---

### 7. Monitor deployment

Open:

https://central.sonatype.com/publishing/deployments

Wait until the deployment status changes from:

```
PUBLISHING
```

to

```
PUBLISHED
```

If validation fails, fix the reported issue and upload a new bundle.

---

## Dependency

After publication, the SDK can be used as:

```gradle
implementation("io.hinkal:hinkal-android:<version>")
```

---
