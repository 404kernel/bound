class Bound < Formula
  desc "Programmable spend vaults and financial rails for autonomous AI agents"
  homepage "https://github.com/404kernel/bound"
  version "0.2.1"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/404kernel/bound/releases/download/v#{version}/bound-darwin-arm64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # Calculated automatically on tap release
    else
      url "https://github.com/404kernel/bound/releases/download/v#{version}/bound-darwin-amd64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/404kernel/bound/releases/download/v#{version}/bound-linux-arm64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    else
      url "https://github.com/404kernel/bound/releases/download/v#{version}/bound-linux-amd64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  def install
    binary_name = OS.mac? ? (Hardware::CPU.arm? ? "bound-darwin-arm64" : "bound-darwin-amd64") : (Hardware::CPU.arm? ? "bound-linux-arm64" : "bound-linux-amd64")
    bin.install binary_name => "bound"
  end

  test do
    system "#{bin}/bound", "help"
  end
end
