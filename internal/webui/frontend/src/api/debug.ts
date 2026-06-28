import { api } from './client'

export function debugMessage(
  platform: string,
  message: string,
  userId = 'debug_user',
  groupId = 'debug_group',
  messageType = 'private',
) {
  return api.request('POST', '/api/debug/message', {
    platform,
    message,
    user_id: userId,
    group_id: groupId,
    message_type: messageType,
  })
}
