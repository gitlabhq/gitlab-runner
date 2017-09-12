#!/usr/bin/env ruby

require 'json'
require 'net/http'
require 'uri'

per_page = 15

private_token = ENV['GITLAB_PRIVATE_TOKEN']
unless private_token && !private_token.empty?
  STDERR.puts 'Please set GITLAB_PRIVATE_TOKEN variable'
  exit 1
end

starting_point = ENV['STARTING_POINT']
if !starting_point || starting_point.empty?
  starting_point = `git describe --tags --abbrev=0 --match "v[0-9].[0-9].[0-9]"`.strip

  STDERR.puts "STARTING_POINT variable not set, using autodiscovered: #{starting_point}"
end
unless starting_point && !starting_point.empty?
  STDERR.puts 'Please set STARTING_POINT variable'
end

exclude_mr_ids = []
exclude_mr_ids = ENV['EXCLUDE_MR_IDS'].split(',').map(&:to_i) if ENV['EXCLUDE_MR_IDS']
project_id = ENV['PROJECT_ID'] || 'gitlab-org%2Fgitlab-runner'

base_url = URI("https://gitlab.com/api/v3/projects/#{project_id}/merge_requests/")
merge_requests = {}

merge_request_ids_cmd = "git log #{starting_point}.. --first-parent | grep -E \"^\\s*See merge request \![0-9]+$\" | grep -Eo \"[0-9]+$\" | xargs echo"
merge_request_ids = `#{merge_request_ids_cmd}`.split(' ').map(&:to_i).reject{ |id| exclude_mr_ids.include?(id) }.reverse
merge_request_ids.sort.each_slice(per_page).to_a.each do |part|
  query = part.map do |id|
    "iid[]=#{id}"
  end

  query << "per_page=#{per_page}"
  query << "private_token=#{private_token}"

  base_url.query = query.join('&')

  data = JSON.parse(Net::HTTP.get(base_url))
  data.each do |mr|
    merge_requests[mr['iid'].to_i] = mr['title']
  end
end

puts
merge_request_ids.each do |iid|
  puts "- #{merge_requests[iid]} !#{iid}"
end
