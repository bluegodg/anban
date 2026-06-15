export function formatLoginError(error) {
  if (error?.status === 401) return '访问码错误，请重新输入';
  if (error?.status && error?.message) return `${error.message}（${error.status}）`;
  return '无法连接安伴服务，请检查后端地址';
}
