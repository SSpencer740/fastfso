# fastFSO Internal Administrator Dashboard
## Feature Summary 
This internal dashboard provides basic administration for customers.  

### Story
As a fastFSO internal admin, I want to have a centralized control panel where I can manage the accounts of our customers so that we can approprately assign roles and onboard customers quickly. 
The controls are as follows:
- Assign user accounts for each user in the company.  A total count will be provided.
- Assign user roles: individual contributor, read only fso, fso, admin
- Assign user sub orginizations.  Eg. Sub groups within a main tenancy.  Users can be a part of multiple sub organizations. 
- Reset password
- Enable/disable AI features:
    - Enable/disable ask/answer bot
    - enable/disable MCP for FSOs, read only, and admin.  IC does not need mcp, but if it's easier to give MCP to all or none, it's fine to provide it. Note: RBAC is very important here.  MCP should not return data for users out of the permission boundry

### Nice to have features
- bulk upload a list of users and account details.