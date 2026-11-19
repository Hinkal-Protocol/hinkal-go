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
libs/go/publish/android/gradle.properties
```

Update:

```properties
sdkVersion=x.y.z (e.x. 0.1.0 -> 0.1.1)
```

---

### 3. Export publishing credentials

Retrieve the publishing credentials from the team's secret manager. Never commit them to this
repository, and prefer loading them from the secret manager over typing them into a shell, so they
do not end up in shell history.

The required environment variables are:

| Variable | Meaning | Secret |
| --- | --- | --- |
| `MAVEN_CENTRAL_USERNAME` | Maven Central Portal token username | yes |
| `MAVEN_CENTRAL_PASSWORD` | Maven Central Portal token password | yes |
| `GPG_SIGNING_KEY` | Exported armoured private signing key | yes |
| `GPG_SIGNING_PASSWORD` | Passphrase protecting that private key | yes |
| `GPG_KEY_FINGERPRINT` | Fingerprint of the signing key, selects which key to export | no |

`GPG_KEY_FINGERPRINT` is the primary key marked `[SC]` in `gpg --list-secret-keys --keyid-format LONG`,
not the `[E]` encryption subkey.

```bash
export MAVEN_CENTRAL_USERNAME="<from secret manager>"
export MAVEN_CENTRAL_PASSWORD="<from secret manager>"

export GPG_KEY_FINGERPRINT="<signing key fingerprint>"
export GPG_SIGNING_KEY="$(gpg --armor --export-secret-keys "$GPG_KEY_FINGERPRINT")"

export GPG_SIGNING_PASSWORD="<from secret manager>"
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

Run the remaining steps from `libs/go/publish/android`:

```bash
cd libs/go/publish/android
./gradlew clean publishToMavenLocal
```

Expected:

```
BUILD SUCCESSFUL
```

---

### 5. Create the signed Maven bundle

```bash
./gradlew clean publishReleasePublicationToLocalStagingRepository
```

Artifacts are created under:

```
build/staging-deploy/
```

---

### 6. Upload to Maven Central

```bash
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
