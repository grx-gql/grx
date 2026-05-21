#!/usr/bin/env ruby
# frozen_string_literal: true

# Validate GitHub issue form YAML under .github/ISSUE_TEMPLATE/
# https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-issue-forms

require "yaml"

ROOT = File.expand_path("../..", __dir__)
TEMPLATE_DIR = File.join(ROOT, ".github", "ISSUE_TEMPLATE")
ISSUE_FORM_TYPES = %w[markdown textarea input dropdown checkboxes].freeze

def fail!(msg)
  warn("error: #{msg}")
  exit 1
end

def validate_issue_form(path, data)
  fail!("#{path}: root must be a Hash") unless data.is_a?(Hash)
  %w[name description body].each do |key|
    fail!("#{path}: missing required key #{key.inspect}") unless data.key?(key)
  end
  body = data["body"]
  fail!("#{path}: 'body' must be a non-empty Array") unless body.is_a?(Array) && !body.empty?

  body.each_with_index do |block, i|
    fail!("#{path}: body[#{i}] must be a Hash") unless block.is_a?(Hash)
    btype = block["type"]
    unless ISSUE_FORM_TYPES.include?(btype)
      fail!(
        "#{path}: body[#{i}] has invalid or missing type #{btype.inspect}; " \
        "expected one of #{ISSUE_FORM_TYPES.sort.inspect}"
      )
    end
    fail!("#{path}: body[#{i}] (#{btype}) missing 'attributes'") if btype != "markdown" && !block.key?("attributes")
  end
end

def validate_config(path, data)
  fail!("#{path}: root must be a Hash") unless data.is_a?(Hash)
  links = data["contact_links"]
  return if links.nil?

  fail!("#{path}: contact_links must be an Array") unless links.is_a?(Array)
  links.each_with_index do |link, i|
    fail!("#{path}: contact_links[#{i}] must be a Hash") unless link.is_a?(Hash)
    %w[name url about].each do |key|
      fail!("#{path}: contact_links[#{i}] missing #{key.inspect}") unless link.key?(key)
    end
  end
end

Dir.chdir(ROOT) do
  fail!("missing directory #{TEMPLATE_DIR}") unless File.directory?(TEMPLATE_DIR)

  yml_files = Dir.glob(File.join(TEMPLATE_DIR, "*.yml")).sort
  fail!("no .yml files under #{TEMPLATE_DIR}") if yml_files.empty?

  yml_files.each do |path|
    data = YAML.load_file(path)
    if File.basename(path) == "config.yml"
      validate_config(path, data)
    else
      validate_issue_form(path, data)
    end
  end

  puts "OK: validated #{yml_files.size} file(s) under #{TEMPLATE_DIR}"
end
