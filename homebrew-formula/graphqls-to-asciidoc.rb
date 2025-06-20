class GraphqlsToAsciidoc < Formula
  desc "Convert GraphQL schema files to comprehensive AsciiDoc documentation"
  homepage "https://github.com/bovinemagnet/graphqls-to-asciidoc"
  version "0.0.9"
  
  if OS.mac?
    if Hardware::CPU.arm?
      url "https://github.com/bovinemagnet/graphqls-to-asciidoc/releases/download/v#{version}/graphqls-to-asciidoc_#{version}_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_DARWIN_ARM64"
    else
      url "https://github.com/bovinemagnet/graphqls-to-asciidoc/releases/download/v#{version}/graphqls-to-asciidoc_#{version}_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_DARWIN_AMD64"
    end
  elsif OS.linux?
    if Hardware::CPU.arm?
      url "https://github.com/bovinemagnet/graphqls-to-asciidoc/releases/download/v#{version}/graphqls-to-asciidoc_#{version}_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_LINUX_ARM64"
    else
      url "https://github.com/bovinemagnet/graphqls-to-asciidoc/releases/download/v#{version}/graphqls-to-asciidoc_#{version}_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_LINUX_AMD64"
    end
  end

  def install
    bin.install "graphqls-to-asciidoc"
  end

  test do
    # Test version output
    assert_match version.to_s, shell_output("#{bin}/graphqls-to-asciidoc -version")
    
    # Test with a simple schema
    (testpath/"test.graphql").write <<~EOS
      """
      A simple test type
      """
      type TestType {
        """
        A test field
        """
        id: ID!
        name: String
      }
      
      type Query {
        """
        Get a test object
        """
        getTest(id: ID!): TestType
      }
    EOS
    
    output = shell_output("#{bin}/graphqls-to-asciidoc -schema #{testpath}/test.graphql")
    assert_match "GraphQL Documentation", output
    assert_match "Query", output
    assert_match "TestType", output
  end
end