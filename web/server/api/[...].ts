export default defineEventHandler(async (event) => {
  const apiGatewayUrl = useRuntimeConfig().proxy.apiGatewayUrl;
  const target = `${apiGatewayUrl}${event.path}`;
  return proxyRequest(event, target);
});
