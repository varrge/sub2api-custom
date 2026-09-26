#!/usr/bin/env ruby
# Release policy regression checks; Ruby's standard library is sufficient.
require 'yaml'
require 'open3'
require 'tmpdir'

root = File.expand_path('../..', __dir__)
ci = YAML.load_file(File.join(root, '.github/workflows/backend-ci.yml'))
release = YAML.load_file(File.join(root, '.github/workflows/release.yml'))

def assert(condition, message)
  raise message unless condition
end

def checkout(job)
  job.fetch('steps').find { |step| step['uses'].to_s.start_with?('actions/checkout@') }
end

def git(dir, *args)
  out, err, status = Open3.capture3('git', '-C', dir, '-c', 'user.name=Release test', '-c', 'user.email=release@example.invalid', '-c', 'core.hooksPath=/dev/null', '-c', 'commit.gpgsign=false', *args)
  assert(status.success?, "Git fixture failed: #{err}")
  out.strip
end

# Psych versions that use YAML 1.1 interpret the key "on" as true.
events = ci.fetch('on', ci[true])
assert(events.key?('workflow_call'), 'Release must reuse the complete CI workflow')
assert(events.fetch('push').fetch('branches').include?('**'), 'Branch CI must remain enabled')
assert(events.fetch('push').fetch('tags-ignore').include?('v*'), 'Release tags must not run duplicate standalone CI')
assert(events.key?('pull_request'), 'Pull request CI must remain enabled')

jobs = ci.fetch('jobs')
gate = jobs.fetch('required')
checks = jobs.keys - ['required']
assert(gate.fetch('needs').sort == checks.sort, 'CI gate must require every CI job')
assert(gate['if'] == '${{ always() }}', 'CI gate must run even after failures or skipped jobs')
checks.each do |name|
  job = jobs.fetch(name)
  assert(checkout(job).fetch('with').fetch('ref') == '${{ inputs.ref || github.sha }}', "#{name} must check the requested commit")
  assert(!job['continue-on-error'], "#{name} cannot ignore failures")
  assert(job.fetch('steps').none? { |step| step['continue-on-error'] }, "#{name} cannot ignore step failures")
end

# Execute the actual gate for success and for each failed/skipped/cancelled dependency.
gate_script = gate.fetch('steps').fetch(0).fetch('run')
assert(gate.fetch('steps').fetch(0).fetch('env').fetch('CI_RESULTS') == "${{ join(needs.*.result, ' ') }}", 'Gate must examine actual job results')
scenarios = [Array.new(checks.length, 'success')]
checks.each_index do |index|
  %w[failure skipped cancelled].each do |result|
    statuses = Array.new(checks.length, 'success')
    statuses[index] = result
    scenarios << statuses
  end
end
scenarios.each do |statuses|
  _, _, status = Open3.capture3({ 'CI_RESULTS' => statuses.join(' ') }, '/bin/bash', '-e', '-c', gate_script)
  assert(status.success? == statuses.all? { |value| value == 'success' }, "CI gate admitted incorrect results: #{statuses}")
end

release_jobs = release.fetch('jobs')
release_ci = release_jobs.fetch('ci')
assert(release_ci['uses'] == './.github/workflows/backend-ci.yml', 'Release must invoke the local CI workflow')
assert(release_ci['needs'] == ['resolve-ref'], 'CI must wait for tag resolution')
assert(release_ci.fetch('with').fetch('ref') == '${{ needs.resolve-ref.outputs.sha }}', 'CI must validate the resolved commit')
publish = release_jobs.fetch('release')
assert(publish.fetch('needs').sort == %w[prepare ci build-binaries].sort, 'Publishing must require CI and all build inputs')
assert(publish['if'] == "${{ needs.ci.result == 'success' }}", 'Publishing must explicitly require successful CI')
assert(release.fetch('permissions') == { 'contents' => 'read' }, 'Preparatory jobs must have read-only permissions')
assert(publish.fetch('permissions') == { 'contents' => 'write', 'packages' => 'write' }, 'Only publishing needs write permissions')
prepare = release_jobs.fetch('prepare')
assert(prepare.fetch('needs') == ['resolve-ref'], 'Matrix preparation must wait for tag resolution')
source_checkout = prepare.fetch('steps').find { |step| step['name'] == 'Checkout selected application source' }
assert(source_checkout.fetch('with').fetch('ref') == '${{ needs.resolve-ref.outputs.sha }}', 'Matrix preparation must use the pinned source')
assert(release_jobs.fetch('build-binaries').fetch('needs').sort == %w[prepare build-frontend].sort, 'Binary builds must require the prepared source and frontend')
assert(!release_jobs.key?('sync-version-file'), 'Release must not overwrite an unrelated default branch')
%w[build-frontend build-binaries release].each do |name|
  job = release_jobs.fetch(name)
  assert(Array(job.fetch('needs')).include?('prepare'), "#{name} must wait for source preparation")
  assert(checkout(job).fetch('with').fetch('ref') == '${{ needs.prepare.outputs.sha }}', "#{name} must build the commit CI validated")
end
goreleaser = publish.fetch('steps').find { |step| step['uses'].to_s.start_with?('goreleaser/goreleaser-action@') }
assert(goreleaser.fetch('env').fetch('GORELEASER_CURRENT_TAG') == '${{ needs.prepare.outputs.tag }}', 'GoReleaser must publish the requested tag even during manual runs')

resolver = release_jobs.fetch('resolve-ref')
validation = resolver.fetch('steps').find { |step| step['id'] == 'tag' }
assert(checkout(resolver).fetch('with').fetch('ref') == '${{ steps.tag.outputs.ref }}', 'Resolver must explicitly check out a tag, never a branch')
Dir.mktmpdir('release-policy') do |dir|
  output = File.join(dir, 'output')
  %w[v1.2.3 v0.2.7-custom.7 v2.0.0-rc.1 main refs/heads/main v1.2.3/branch v1.2.3..oops].concat(['', "v1.2.3\nsha=bad", 'v1.2.3;echo injected', '$(exit 0)']).each do |tag|
    _, _, status = Open3.capture3({ 'RELEASE_TAG' => tag, 'GITHUB_OUTPUT' => output }, '/bin/bash', '-e', '-c', validation.fetch('run'))
    assert(status.success? == %w[v1.2.3 v0.2.7-custom.7 v2.0.0-rc.1].include?(tag), "Incorrect tag validation: #{tag.inspect}")
  end
end

# Dry runs may select a branch while real publication remains tag-only.
Dir.mktmpdir('release-dry-run-policy') do |dir|
  output = File.join(dir, 'output')
  _, _, status = Open3.capture3({ 'RELEASE_TAG' => 'custom/0.2.8', 'DRY_RUN' => 'true', 'GITHUB_OUTPUT' => output }, '/bin/bash', '-e', '-c', validation.fetch('run'))
  assert(status.success? && File.read(output).include?('ref=custom/0.2.8'), 'Dry runs must support candidate branches')
end
steps = publish.fetch('steps')
verify_index = steps.index { |step| step['name'] == 'Verify release tag still points to the validated commit' }
images_index = steps.index { |step| step['name'] == 'Build images and publish manifests' }
release_index = steps.index(goreleaser)
assert(verify_index < images_index && verify_index < release_index, 'Tag verification must precede every publication step')
assert(steps[verify_index]['if'] == "${{ env.DRY_RUN != 'true' }}", 'Branch dry runs must not require a release tag')
assert(publish.fetch('env').fetch('RELEASE_SHA') == '${{ needs.prepare.outputs.sha }}', 'Publication must verify the prepared source SHA')

# Execute the workflow's own pin and publication checks against real Git refs.
pin_script = resolver.fetch('steps').find { |step| step['id'] == 'commit' }.fetch('run')
verify_script = publish.fetch('steps').find { |step| step['name'] == 'Verify release tag still points to the validated commit' }.fetch('run')
Dir.mktmpdir('release-refs') do |dir|
  origin = File.join(dir, 'origin')
  clone = File.join(dir, 'clone')
  Dir.mkdir(origin)
  git(origin, 'init', '--quiet')
  git(origin, 'commit', '--quiet', '--allow-empty', '-m', 'Release commit')
  release_sha = git(origin, 'rev-parse', 'HEAD')
  git(origin, 'tag', 'v1.2.3')
  git(origin, '-c', 'tag.gpgsign=false', 'tag', '-a', 'v1.2.4', '-m', 'Annotated release')
  git(origin, 'commit', '--quiet', '--allow-empty', '-m', 'Different dispatch branch commit')
  branch_sha = git(origin, 'rev-parse', 'HEAD')
  git(dir, 'clone', '--quiet', origin, clone)
  git(clone, 'checkout', '--quiet', '--detach', release_sha)
  output = File.join(dir, 'output')
  env = { 'GITHUB_OUTPUT' => output, 'GITHUB_SHA' => branch_sha, 'GITHUB_EVENT_NAME' => 'workflow_dispatch' }
  _, _, status = Open3.capture3(env, '/bin/bash', '-e', '-c', pin_script, chdir: clone)
  assert(status.success? && File.read(output).include?("sha=#{release_sha}"), 'Manual release must pin the tag commit even when the dispatch branch differs')
  _, _, status = Open3.capture3(env.merge('GITHUB_EVENT_NAME' => 'push'), '/bin/bash', '-e', '-c', pin_script, chdir: clone)
  assert(!status.success?, 'Tag push must reject a moved source ref')
  %w[v1.2.3 v1.2.4].each do |tag|
    env = { 'RELEASE_TAG' => tag, 'RELEASE_SHA' => release_sha }
    _, _, status = Open3.capture3(env, '/bin/bash', '-e', '-c', verify_script, chdir: clone)
    assert(status.success?, "Unchanged lightweight/annotated tag should publish: #{tag}")
    git(origin, 'tag', '--force', tag, branch_sha)
    _, _, status = Open3.capture3(env, '/bin/bash', '-e', '-c', verify_script, chdir: clone)
    assert(!status.success?, 'Publishing must reject a tag moved since CI')
    git(origin, 'tag', '--delete', tag)
    _, _, status = Open3.capture3(env, '/bin/bash', '-e', '-c', verify_script, chdir: clone)
    assert(!status.success?, 'Publishing must reject a deleted tag even if a local tag remains')
  end
end

puts "release workflow tests passed (#{scenarios.length} CI result scenarios, tag validation, source pinning and publication dependencies)"
