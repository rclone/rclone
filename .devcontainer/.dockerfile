ARG BASE_IMAGE=ubuntu/go
FROM ${BASE_IMAGE}
ARG USER_CI=jenkinsci
ARG USER_UID
ARG USER_GID
# Install required plugin development package
# Create CI USer
RUN <<EOT
# Remove existing user if any
id ${USER_UID} && userdel -rf $(id -nu ${USER_UID})
groupadd -fg ${USER_GID} ${USER_CI} && 
    useradd -ms /bin/bash -g ${USER_GID} -u ${USER_UID} ${USER_CI}
EOT
# Turn it sudoer
COPY <<EOT /etc/sudoers.d/${USER_CI}
${USER_CI} ALL=(root) NOPASSWD:ALL
EOT
# Activate user
USER ${USER_CI}
# Add cool prompt
RUN <<EOT
cat <<"EOF" >> /home/${USER_CI}/.bashrc
# bash theme - partly inspired by https://github.com/ohmyzsh/ohmyzsh/blob/master/themes/robbyrussell.zsh-theme
__bash_prompt() {
    local userpart='`export XIT=$? \
        && [ ! -z "${GITHUB_USER:-}" ] && echo -n "\[\033[0;32m\]@${GITHUB_USER:-} " || echo -n "\[\033[0;32m\]\u " \
        && [ "$XIT" -ne "0" ] && echo -n "\[\033[1;31m\]➜" || echo -n "\[\033[0m\]➜"`'
    local gitbranch='`\
        if [ "$(git config --get devcontainers-theme.hide-status 2>/dev/null)" != 1 ] && [ "$(git config --get codespaces-theme.hide-status 2>/dev/null)" != 1 ]; then \
            export BRANCH="$(git --no-optional-locks symbolic-ref --short HEAD 2>/dev/null || git --no-optional-locks rev-parse --short HEAD 2>/dev/null)"; \
            if [ "${BRANCH:-}" != "" ]; then \
                echo -n "\[\033[0;36m\](\[\033[1;31m\]${BRANCH:-}" \
                && if [ "$(git config --get devcontainers-theme.show-dirty 2>/dev/null)" = 1 ] && \
                    git --no-optional-locks ls-files --error-unmatch -m --directory --no-empty-directory -o --exclude-standard ":/*" > /dev/null 2>&1; then \
                        echo -n " \[\033[1;33m\]✗"; \
                fi \
                && echo -n "\[\033[0;36m\]) "; \
            fi; \
        fi`'
    local lightblue='\[\033[1;34m\]'
    local removecolor='\[\033[0m\]'
    PS1="${userpart} ${lightblue}\w ${gitbranch}${removecolor}\$ "
    unset -f __bash_prompt
}
__bash_prompt
EOF
EOT
# Add maven settings for jenkins ci
RUN mkdir -p /home/${USER_CI}/.m2
