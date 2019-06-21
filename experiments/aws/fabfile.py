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
def send_config(conn):
    ''' Sends keys, addresses, and parameters. '''
    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    conn.put('data/config.json', repo_path)


@task 
def send_data(conn, pid):
    ''' Sends keys, addresses, and parameters. '''
    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    conn.put(f'data/{pid}.keys', repo_path)

    conn.put('data/config.json', repo_path)

@task
def send_repo(conn):
    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    conn.put('repo.zip', repo_path)
    conn.run(f'rm -rf {repo_path}/pkg {repo_path}/cmd')
    conn.run(f'unzip -q {repo_path}/repo.zip -d {repo_path}')

#======================================================================================
#                                   run experiments
#======================================================================================

@task
def run_protocol(conn, pid, delay='0'):
    ''' Runs the protocol.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        if int(pid) in list(range(16)):
            cmd = f'go run cmd/gomel/main.go --poset {pid}.poset --keys {pid}.keys --config config.json --db pkg/testdata/users.txt --delay {int(float(delay))} --cpuprof cpuprof --memprof memprof '
        else:
            cmd = f'go run cmd/gomel/main.go --poset {pid}.poset --keys {pid}.keys --config config.json --db pkg/testdata/users.txt --delay {int(float(delay))}'
        conn.run(f'PATH="$PATH:/snap/bin" && dtach -n `mktemp -u /tmp/dtach.XXXX` {cmd}')

@task
def run_protocol_profiler(conn, pid, delay='0'):
    ''' Runs the protocol.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        cmd = f'go run cmd/gomel/main.go --poset {pid}.poset --keys {pid}.keys --config config.json --db pkg/testdata/users.txt --delay {int(float(delay))}'
        conn.run(f'PATH="$PATH:/snap/bin" && dtach -n `mktemp -u /tmp/dtach.XXXX` {cmd}')

@task
def get_profile_data(conn, pid):
    ''' Retrieves aleph.log from the server.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    conn.get(f'{repo_path}/cpuprof', f'../results/{pid}.cpuprof')
    conn.get(f'{repo_path}/memprof', f'../results/{pid}.memprof')

@task
def get_poset(conn, pid):
    ''' Retrieves aleph.log from the server.'''


    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        conn.run(f'zip -q {pid}.poset.zip {pid}.poset')
    conn.get(f'{repo_path}/{pid}.poset.zip', f'../results/{pid}.poset.zip')


@task
def get_log(conn, pid):
    ''' Retrieves aleph.log from the server.'''


    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        conn.run(f'mv aleph.log {pid}.log')
        conn.run(f'zip -q {pid}.log.zip {pid}.log')
    conn.get(f'{repo_path}/{pid}.log.zip', f'../results/{pid}.log.zip')

@task
def stop_world(conn):
    ''' Kills the committee member.'''

    conn.run('killall go')
    conn.run('killall main')


@task
def version(conn):
    ''' Always changing task for experimenting with fab.'''

    conn.run(f'PATH="$PATH:/snap/bin" && go version')
