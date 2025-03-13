RUN pipenv install --ignore-pipfile --system --deploy --clear \
 && pip uninstall pipenv -y \
 && rm -rf /root/.cache