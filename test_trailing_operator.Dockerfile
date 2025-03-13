RUN pipenv install --ignore-pipfile --system --deploy --clear \
&& pip uninstall pipenv -y \
&& apt-get autoremove -y \
&& rm -rf /root/.cache \
&& apt-get remove -y gcc libpq-dev \
&& apt-get clean 