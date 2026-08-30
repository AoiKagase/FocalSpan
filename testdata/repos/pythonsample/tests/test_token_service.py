import pytest
from src.auth.token_service import TokenService

@pytest.mark.parametrize("value", ["expired"])
def test_expired_token(service):
    assert not service.validate_token(service)

@pytest.fixture
def service():
    return TokenService()
