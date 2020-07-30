'''Routines called by fab. Assumes that all are called from */experiments/aws.'''

from fabric import task


#======================================================================================
#                                    installation
#======================================================================================

@task
def setup(conn):
    ''' Install dependencies in a nonblocking way.'''

    conn.put('setup.sh', '.')
    conn.sudo('apt update', hide='both')
    conn.sudo('apt install dtach', hide='both')
    conn.run('PATH="$PATH:/snap/bin" && dtach -n `mktemp -u /tmp/dtach.XXXX` bash setup.sh', hide='both')

@task
def setup_completed(conn):
    ''' Check if installation completed.'''

    result = conn.run('tail -1 setup.log')
    return result.stdout.strip()

@task
def clone_repo(conn):
    '''Clones main repo.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    # delete current repo
    conn.run(f'rm -rf {repo_path}')
    # clone using deployment token
    user_token = 'gitlab+deploy-token-70309:G2jUsynd3TQqsvVfn4T7'
    conn.run(f'git clone http://{user_token}@gitlab.com/alephledger/consensus-go.git {repo_path}')

@task
def build_gomel(conn):
    conn.run(f'PATH="$PATH:/snap/bin" && go build /home/ubuntu/go/src/gitlab.com/alephledger/consensus-go/cmd/gomel')

@task
def inst_deps(conn):
    conn.put('deps.sh', '.')
    conn.run('PATH="$PATH:/snap/bin" && dtach -n `mktemp -u /tmp/dtach.XXXX` bash deps.sh', hide='both')

#======================================================================================
#                                   syncing local version
#======================================================================================

@task
def send_keys_addrs(conn):
    ''' Sends keys and addresses, and fixes ip address. '''
    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    conn.put('data/committee.ka', repo_path+'/ka')
    with conn.cd(repo_path):
        conn.run(f'sed s/{conn.host}/$(hostname --ip-address)/g < ka > committee.ka')

@task
def send_data(conn, pid):
    ''' Sends keys, addresses, and parameters. '''
    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    conn.put(f'data/{pid}.pk', repo_path)
    send_keys_addrs(conn)

@task
def send_repo(conn):
    project_path = '/home/ubuntu/go/src/gitlab.com/alephledger'
    conn.put('core-repo.zip', project_path)
    conn.run(f'rm -rf {project_path}/core-go')
    conn.run(f'unzip -q {project_path}/core-repo.zip -d {project_path}')

    repo_path = project_path + '/consensus-go'
    conn.put('consensus-repo.zip', repo_path)
    conn.run(f'rm -rf {repo_path}/pkg {repo_path}/cmd')
    conn.run(f'unzip -q {repo_path}/consensus-repo.zip -d {repo_path}')

#======================================================================================
#                                   run experiments
#======================================================================================

@task
def run_protocol(conn, pid, delay='0'):
    ''' Runs the protocol.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        conn.run(f'PATH="$PATH:/snap/bin" && go build {repo_path}/cmd/gomel')
        cmd = f'./gomel --priv {pid}.pk\
                    --keys_addrs committee.ka\
                    --delay {int(float(delay))}\
                    --setup {delay == "0"}'
        conn.run(f'dtach -n `mktemp -u /tmp/dtach.XXXX` {cmd}')

@task
def run_protocol_profiler(conn, pid, delay='0'):
    ''' Runs the protocol.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        conn.run(f'PATH="$PATH:/snap/bin" && go build {repo_path}/cmd/gomel')
        cmd = f'./gomel --priv {pid}.pk\
                    --keys_addrs committee.ka\
                    --delay {int(float(delay))}\
                    --setup {"true" if delay=="0" else "false"}'
        if int(pid) % 16 == 0 :
            cmd += ' --cpuprof cpuprof --memprof memprof --mf 5 --bf 0'
        conn.run(f'dtach -n `mktemp -u /tmp/dtach.XXXX` {cmd}')

@task
def stop_world(conn):
    ''' Kills the committee member.'''

    conn.run('pkill --signal ABRT -f gomel')

#======================================================================================
#                                        get data
#======================================================================================

@task
def get_profile(conn, pid):
    ''' Retrieves cpuprof and memprof from the server.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        conn.run(f'cp cpuprof {pid}.cpuprof')
        conn.run(f'cp memprof {pid}.memprof')
        conn.run(f'zip -q prof.zip {pid}.cpuprof {pid}.memprof')
    conn.get(f'{repo_path}/prof.zip', f'../results/{pid}.prof.zip')

@task
def get_dag(conn, pid):
    ''' Retrieves aleph.dag from the server.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        conn.run(f'zip -q {pid}.dag.zip {pid}.dag')
    conn.get(f'{repo_path}/{pid}.dag.zip', f'../results/{pid}.dag.zip')

@task
def get_log(conn, pid):
    ''' Retrieves aleph.log from the server.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        conn.run(f'zip -q {pid}.logs.zip {pid}.json {pid}.setup.json')
    conn.get(f'{repo_path}/{pid}.logs.zip', f'../results/{pid}.log.zip')

#======================================================================================
#                                        misc
#======================================================================================

@task
def test(conn):
    ''' Tests if connection is ready '''

    conn.open()
