/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  transpilePackages: ['@patternfly/react-core', '@patternfly/react-styles', '@patternfly/react-table'],
  experimental: {
    // Needs to be excluded from webpack and required by chromadb
    serverComponentsExternalPackages: ['sharp', 'onnxruntime-node'],
  },
};

module.exports = nextConfig;
