class Kggraph < Formula
  desc "Knowledge graph CLI and MCP server for agents"
  homepage "https://github.com/OctoSucker/KGgraph"
  url "https://github.com/OctoSucker/KGgraph/archive/refs/tags/v0.1.2.tar.gz"
  sha256 "REPLACE_WITH_SOURCE_TARBALL_SHA256"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"kggraph"), "./cmd/kggraph"
  end

  test do
    output = shell_output("#{bin}/kggraph list-nodes --db #{testpath}/kggraph.sqlite")
    assert_match "nodes", output
  end
end
