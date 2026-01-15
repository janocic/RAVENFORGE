# Maintainer: Your Name <your.email@example.com>
pkgname=ravenforge
pkgver=1.0.0
pkgrel=1
pkgdesc="Security Operations Automation Platform"
arch=('x86_64')
url="https://github.com/janocic/ravenforge"
license=('MIT')
depends=('docker' 'sqlite' 'python' 'python-pip')
makedepends=('go' 'git')
backup=('etc/ravenforge/ravenforge.yaml')
source=("$pkgname-$pkgver.tar.gz::https://github.com/janocic/ravenforge/archive/v$pkgver.tar.gz")
sha256sums=('SKIP')

build() {
    cd "$pkgname-$pkgver/core"
    
    export CGO_CPPFLAGS="${CPPFLAGS}"
    export CGO_CFLAGS="${CFLAGS}"
    export CGO_CXXFLAGS="${CXXFLAGS}"
    export CGO_LDFLAGS="${LDFLAGS}"
    export GOFLAGS="-buildmode=pie -trimpath -ldflags=-linkmode=external -mod=readonly -modcacherw"
    
    go build -o ravenforged ./cmd/ravenforged
    go build -o ravenforge ./cmd/ravenforge
}

package() {
    cd "$pkgname-$pkgver"
    
    # Install binaries
    install -Dm755 core/ravenforged "$pkgdir/usr/bin/ravenforged"
    install -Dm755 core/ravenforge "$pkgdir/usr/bin/ravenforge"
    
    # Install config
    install -Dm644 core/config/ravenforge.linux.yaml "$pkgdir/etc/ravenforge/ravenforge.yaml"
    
    # Install systemd service
    install -Dm644 scripts/ravenforged.service "$pkgdir/usr/lib/systemd/system/ravenforged.service"
    
    # Install tools
    install -dm755 "$pkgdir/usr/share/ravenforge/tools"
    cp -r tools/* "$pkgdir/usr/share/ravenforge/tools/"
    
    # Install SDK
    install -dm755 "$pkgdir/usr/share/ravenforge/sdk"
    cp -r sdk/* "$pkgdir/usr/share/ravenforge/sdk/"
    
    # Install documentation
    install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
    install -Dm644 docs/ARCHITECTURE.md "$pkgdir/usr/share/doc/$pkgname/ARCHITECTURE.md"
    install -Dm644 docs/TOOL_DEVELOPMENT.md "$pkgdir/usr/share/doc/$pkgname/TOOL_DEVELOPMENT.md"
    
    # Install license
    install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
    
    # Create data directories (will be created by systemd tmpfiles)
    install -dm750 "$pkgdir/var/lib/ravenforge"
    install -dm750 "$pkgdir/var/lib/ravenforge/artifacts"
    install -dm750 "$pkgdir/var/log/ravenforge"
}

post_install() {
    # Create ravenforge user if it doesn't exist
    if ! getent passwd ravenforge > /dev/null; then
        useradd -r -s /bin/false -d /var/lib/ravenforge ravenforge
    fi
    
    # Set permissions
    chown -R ravenforge:ravenforge /var/lib/ravenforge
    chown -R ravenforge:ravenforge /var/log/ravenforge
    
    echo "==> RavenForge installed successfully!"
    echo "==> To start the service: systemctl start ravenforged"
    echo "==> To enable on boot: systemctl enable ravenforged"
    echo "==> Configuration: /etc/ravenforge/ravenforge.yaml"
}

post_upgrade() {
    post_install
}
