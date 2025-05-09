const fs = require('fs');
const dockerfileContent = fs.readFileSync('test-dockerfile.txt', 'utf8');

// Convert dockerfile request
const convertRequest = {
  jsonrpc: "2.0",
  id: "test-1",
  method: "mcp.call_tool",
  params: {
    name: "convert_dockerfile",
    arguments: {
      dockerfile_content: dockerfileContent,
      organization: "example"
    }
  }
};

fs.writeFileSync('test-request.json', JSON.stringify(convertRequest, null, 2));

// Analyze dockerfile request
const analyzeRequest = {
  jsonrpc: "2.0",
  id: "test-2",
  method: "mcp.call_tool",
  params: {
    name: "analyze_dockerfile",
    arguments: {
      dockerfile_content: dockerfileContent
    }
  }
};

fs.writeFileSync('analyze-request.json', JSON.stringify(analyzeRequest, null, 2));

// Healthcheck request
const healthcheckRequest = {
  jsonrpc: "2.0",
  id: "test-3",
  method: "mcp.call_tool",
  params: {
    name: "healthcheck",
    arguments: {}
  }
};

fs.writeFileSync('healthcheck-request.json', JSON.stringify(healthcheckRequest, null, 2));
