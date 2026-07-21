import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import sub2apiProviders from './sub2apiProviders'
import audit from './audit'
import promptAudit from './promptAudit'

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...sub2apiProviders,
  ...audit,
  ...promptAudit,
}
