text = "
def phantom
end
"
require_relative 'dep'
def real
  text = 'target()'
end
it 'works' do
  real()
end
