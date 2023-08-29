export default function renderAvatarHTML(user) {
  return user.avatarURL !== null
    ? `<img class="avatar" src="${user.avatarURL}" `
    : `<span class="avatar" data-initial="${user.username[0]}"></span>`
}