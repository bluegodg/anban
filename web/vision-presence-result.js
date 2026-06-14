export function formatVisionPresenceResult(result = {}) {
  const observation = result.observation || {};
  const presence = presenceLabel(observation.presence);
  const greetingStatus = observation.greeting?.status || '';

  if (greetingStatus === 'pending') {
    return {
      label: '在线',
      detail: '视觉触发的主动问候已排队',
      notice: '视觉触发已排队',
      output: `视觉触发结果：${presence} · 问候已排队`,
    };
  }

  if (observation.triggeredGreeting) {
    return {
      label: '在线',
      detail: '视觉触发了一次主动问候',
      notice: '视觉触发已完成',
      output: `视觉触发结果：${presence} · 已触发问候`,
    };
  }

  return {
    label: '在线',
    detail: '刚刚完成一次视觉判定',
    notice: '视觉判定已返回',
    output: `视觉触发结果：${presence} · 未触发问候`,
  };
}

function presenceLabel(presence) {
  if (presence === 'someone') return '有人';
  if (presence === 'no_one') return '无人';
  return '未知';
}
