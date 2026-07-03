# Homebrew formula for Klip (macOS + Linux).
#
# This builds from source on the user's machine, so on macOS the resulting
# binary is NOT quarantined and Gatekeeper never blocks it — no "Open Anyway".
#
# To publish: copy this file to a tap repo named `homebrew-tap`
# (e.g. github.com/PatrykDz95/homebrew-tap) as `Formula/klip.rb`.
# Users then run:  brew install PatrykDz95/tap/klip
#
# Note: Pro license store IDs are injected only in official release builds,
# so a Homebrew (source) build runs the free tier.
class Klip < Formula
  desc "Secure P2P clipboard sharing and file transfer across devices on your LAN"
  homepage "https://klip-it.app"
  url "https://github.com/PatrykDz95/Klip/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "687f541ccc61cbfa2d5afc6659d03b1471cc3d2a48418cadf5f2a5459e750ba0"
  license "GPL-3.0-or-later"
  head "https://github.com/PatrykDz95/Klip.git", branch: "master"

  depends_on "go" => :build

  on_linux do
    depends_on "pkg-config" => :build
    depends_on "gtk+3"
    depends_on "libayatana-appindicator"
  end

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/klip"
  end

  test do
    # Klip is a system-tray app with no headless mode, so we only verify the
    # binary was produced and is runnable.
    assert_predicate bin/"klip", :executable?
  end
end
